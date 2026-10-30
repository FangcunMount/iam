"""Exercise the actual export workflow with isolated registry/engine fixtures; no Docker daemon."""

import copy, hashlib, json, os, pathlib, subprocess, tempfile

ROOT = pathlib.Path(__file__).resolve().parents[2]
repo = "iam" if (ROOT / "go.mod").is_file() else "qs-operating-system"
roots = [(repo, str(ROOT), "f" * 40)]


def encoded(d):
    return json.dumps(d, sort_keys=True, separators=(",", ":")).encode()


def dg(b):
    return "sha256:" + hashlib.sha256(b).hexdigest()


for repo, root, sha in roots:
    text = pathlib.Path(root, ".github/workflows/cd.yml").read_text()
    start = text.index(
        "      - name: Export the verified frozen candidate without rebuilding\n"
    )
    start = text.index("        run: |\n", start) + len("        run: |\n")
    end = text.index("\n      - name:", start)
    script = "\n".join(line[10:] for line in text[start:end].splitlines())
    with tempfile.TemporaryDirectory() as tmp:
        p = pathlib.Path(tmp)
        tool = p / "docker"
        tool.write_text("""#!/usr/bin/env python3
import json,os,pathlib,sys
args=sys.argv[1:]
with open(os.environ['CALLS'],'a') as f:f.write(json.dumps(args)+'\\n')
if args[:4]==['buildx','imagetools','inspect','--raw']:
 data=json.loads(os.environ['MANIFESTS']);raw=pathlib.Path(data[args[-1].rsplit('@',1)[1]]).read_bytes();sys.stdout.buffer.write(raw+b'\\n')
elif args[:2]==['image','inspect']:
 print(os.environ['LOCAL_IMAGE'])
elif args[0]=='save':sys.stdout.buffer.write(b'fixture')
elif args[0] not in ('pull','tag'):raise SystemExit(3)
""")
        tool.chmod(0o755)
        for name in [
            "config-id",
            "manifest-id",
            "index-id",
            "index-child-id",
            "wrong-id",
            "wrong-config",
            "wrong-repo",
            "wrong-descriptor",
            "wrong-platform",
            "wrong-revision",
            "tampered-manifest",
            "ambiguous-platform",
        ]:
            expected = "sha256:" + "b" * 64
            manifest = {
                "schemaVersion": 2,
                "config": {
                    "digest": ("sha256:" + "c" * 64)
                    if name == "wrong-config"
                    else expected
                },
                "layers": [],
            }
            raw = encoded(manifest)
            child = dg(raw)
            childfile = p / "child"
            childfile.write_bytes(raw)
            mapping = {child: str(childfile)}
            wanted = child
            index = name in ("index-id", "index-child-id", "ambiguous-platform")
            if index:
                item = {
                    "digest": child,
                    "platform": {"os": "linux", "architecture": "amd64"},
                }
                entries = (
                    [item, copy.deepcopy(item)]
                    if name == "ambiguous-platform"
                    else [item]
                )
                parent = encoded({"schemaVersion": 2, "manifests": entries})
                wanted = dg(parent)
                parentfile = p / "parent"
                parentfile.write_bytes(parent)
                mapping[wanted] = str(parentfile)
            if name == "tampered-manifest":
                childfile.write_bytes(raw + b" ")
            image_repo = "registry.example/unit/" + (
                "iam" if repo == "iam" else "qs-operating-system"
            )
            ref = image_repo + "@" + wanted
            local = {
                "Id": expected
                if name == "config-id"
                else (child if name == "index-child-id" else wanted),
                "Os": "linux",
                "Architecture": "amd64",
                "RepoDigests": [ref],
                "Descriptor": {"digest": wanted},
                "Config": {"Labels": {"org.opencontainers.image.revision": sha}},
            }
            if name == "wrong-id":
                local["Id"] = "sha256:" + "d" * 64
            if name == "wrong-repo":
                local["RepoDigests"] = ["other.example/unrelated@" + wanted]
            if name == "wrong-descriptor":
                local["Descriptor"]["digest"] = "sha256:" + "d" * 64
            if name == "wrong-platform":
                local["Architecture"] = "arm64"
            if name == "wrong-revision":
                local["Config"]["Labels"]["org.opencontainers.image.revision"] = (
                    "e" * 40
                )
            calls = p / "calls"
            calls.unlink(missing_ok=True)
            env = {
                **os.environ,
                "PATH": str(p) + ":" + os.environ["PATH"],
                "ALIYUN_ACR_REGISTRY": "registry.example",
                "ALIYUN_ACR_NAMESPACE": "unit",
                "DEPLOY_SHA": sha,
                "CANDIDATE_DIGEST": wanted,
                "CANDIDATE_CONFIG_ID": expected,
                "DEPLOY_IMAGE_PACKAGE": str(p / "image.tar.gz"),
                "CALLS": str(calls),
                "MANIFESTS": json.dumps(mapping),
                "LOCAL_IMAGE": json.dumps([local]),
                "SERVICE": "apiserver" if repo == "iam" else "ops",
            }
            result = subprocess.run(
                ["bash", "-c", script], cwd=root, env=env, capture_output=True
            )
            success = name in ("config-id", "manifest-id", "index-id", "index-child-id")
            assert (result.returncode == 0) == success, (
                repo,
                name,
                result.stderr.decode(),
            )
            log = [json.loads(s) for s in calls.read_text().splitlines()]
            assert any(a[0] == "pull" and "--platform" in a and ref in a for a in log)
            assert any(a[0] == "tag" for a in log) == success
            assert any(a[0] == "save" for a in log) == success
            print(json.dumps({"repo": repo, "case": name, "passed": True}))


# Execute the actual status script: a frozen-image reuse skips the build job,
# while a successful deployment and plan are still mandatory.
import re
import textwrap

workflow = (ROOT / ".github/workflows/cd.yml").read_text()
start = workflow.index("      - name: Log deployment status\n")
start = workflow.index("        run: |\n", start) + len("        run: |\n")
lines = []
for line in workflow[start:].splitlines():
    if line.strip() and not line.startswith("          "):
        break
    lines.append(line)
status_script = textwrap.dedent("\n".join(lines))
for paused, reuse, plan, build, deployment, expected in [
    ("false", "false", "success", "success", "success", True),
    ("false", "true", "success", "skipped", "success", True),
    ("false", "true", "failure", "skipped", "success", False),
    ("false", "false", "success", "skipped", "success", False),
    ("false", "true", "success", "failure", "success", False),
    ("false", "true", "success", "cancelled", "success", False),
    ("false", "true", "success", "skipped", "skipped", False),
    ("false", "true", "success", "skipped", "failure", False),
    ("true", "false", "success", "success", "skipped", True),
    ("true", "true", "success", "skipped", "skipped", True),
    ("true", "false", "success", "failure", "skipped", False),
    ("true", "true", "failure", "skipped", "skipped", False),
]:
    inputs = {
        "vars.PRODUCTION_DEPLOY_PAUSED": paused,
        "needs.plan.outputs.reuse": reuse,
        "needs.plan.result": plan,
        "needs.docker.result": build,
        "needs.deploy.result": deployment,
    }
    rendered = re.sub(r"\$\{\{\s*(.*?)\s*\}\}", lambda m: inputs[m[1]], status_script)
    result = subprocess.run(["bash", "-c", rendered], capture_output=True, text=True)
    assert (result.returncode == 0) == expected, (repo, inputs, result.stdout)
    print(repo, "status", inputs, "passed")

# Exercise the actual isolated-authentication setup, including plugin discovery.
auth_anchor = workflow.index(
    'echo "DOCKER_CONFIG=${DOCKER_CONFIG_DIR}" >> "${GITHUB_ENV}"'
)
setup_start = workflow.rfind("        run: |\n", 0, auth_anchor) + len(
    "        run: |\n"
)
setup_end = workflow.index("\n      - name:", setup_start)
setup = textwrap.dedent(workflow[setup_start:setup_end])
for mode in ["global", "user-only", "missing", "broken-path", "info-failure"]:
    with tempfile.TemporaryDirectory() as temporary:
        directory = pathlib.Path(temporary)
        plugin = directory / "installed plugin with spaces"
        if mode != "broken-path":
            plugin.write_text("#!/bin/sh\nexit 0\n")
            plugin.chmod(0o755)
        docker = directory / "docker"
        docker.write_text("""#!/usr/bin/env python3
import json,os,pathlib,sys
args=sys.argv[1:];config=pathlib.Path(args[1]);args=args[2:]
with open(os.environ['PLUGIN_CALLS'],'a') as f:f.write(json.dumps(args)+'\\n')
mode=os.environ['PLUGIN_MODE']
if args==['buildx','version']:
 ready=mode=='global' or (config/'cli-plugins/docker-buildx').is_file()
 raise SystemExit(0 if ready else 1)
if args==['info','--format','{{json .ClientInfo.Plugins}}']:
 if mode=='info-failure':raise SystemExit(1)
 print(json.dumps([] if mode=='missing' else [{'Name':'buildx','Path':os.environ['PLUGIN_PATH']}]))
else:raise SystemExit(2)
""")
        docker.chmod(0o755)
        env = dict(
            os.environ,
            PATH=str(directory) + os.pathsep + os.environ["PATH"],
            RUNNER_TEMP=str(directory),
            GITHUB_RUN_ID="case",
            GITHUB_ENV=str(directory / "github-env"),
            PLUGIN_MODE=mode,
            PLUGIN_PATH=str(plugin),
            PLUGIN_CALLS=str(directory / "calls"),
            ALIYUN_ACR_REGISTRY="registry.test",
            ALIYUN_ACR_USERNAME="fixture-user",
            ALIYUN_ACR_PASSWORD="private-plugin-fixture",
            GHCR_REGISTRY="ghcr.test",
            GHCR_USERNAME="fixture-user",
            GHCR_TOKEN="private-plugin-fixture",
        )
        result = subprocess.run(
            ["bash", "-c", setup], env=env, capture_output=True, text=True
        )
        assert (result.returncode == 0) == (mode in ["global", "user-only"]), (
            repo,
            mode,
            result.stderr,
        )
        assert "private-plugin-fixture" not in result.stdout + result.stderr
        isolated = directory / "docker-config-case"
        auth = isolated / "config.json"
        assert auth.stat().st_mode & 0o077 == 0
        data = json.loads(auth.read_text())
        assert set(data) == {"auths"}
        assert set(data["auths"]) == (
            {"registry.test", "ghcr.test"} if repo == "iam" else {"registry.test"}
        )
        assert (
            "DOCKER_CONFIG=" + str(isolated) in (directory / "github-env").read_text()
        )
        if mode == "user-only":
            assert (isolated / "cli-plugins/docker-buildx").resolve() == plugin
        if mode == "global":
            calls = [
                json.loads(x) for x in (directory / "calls").read_text().splitlines()
            ]
            assert all(c[0] == "buildx" for c in calls)
        print(repo, "isolated-plugin", mode, "passed")
