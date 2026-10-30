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
