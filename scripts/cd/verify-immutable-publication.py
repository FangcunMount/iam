"""Read an IAM SHA publication from three registries using existing Docker authentication."""
import argparse
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from hashlib import sha256
import json
import re
import subprocess


PLATFORM = ("linux", "amd64")
SHA = re.compile(r"[0-9a-f]{40}")
DIGEST = re.compile(r"sha256:[0-9a-f]{64}")
REPO = re.compile(r"[a-z0-9.:-]+(?:/[a-z0-9._-]+)+")
REVISION = "org.opencontainers.image.revision"


def digest(raw):
    return "sha256:" + sha256(raw).hexdigest()


def platform(row):
    return row.get("os"), row.get("architecture")


def inspect(reference, *options):
    result = subprocess.run(
        ["docker", "buildx", "imagetools", "inspect", *options, reference],
        capture_output=True, timeout=22,
    )
    if result.returncode:
        raise ValueError("registry inspection failed")
    if not 0 < len(result.stdout) <= 512 * 1024:
        raise ValueError("unexpected registry response length")
    return result.stdout


def exact_manifest(reference, expected):
    raw = inspect(reference, "--raw")
    # Buildx emits raw content, with some releases adding a final output newline.
    if digest(raw) != expected and not (raw.endswith(b"\n") and digest(raw[:-1]) == expected):
        raise ValueError("manifest body disagrees with the resolved digest")
    return json.loads(raw)


def read_registry(revision, item):
    name, repository = item
    reference = f"{repository}:{revision}"
    try:
        tag = json.loads(inspect(reference, "--format", "{{json .Manifest}}"))
        top = tag["digest"]
        if not DIGEST.fullmatch(top):
            raise ValueError("invalid published digest")
        document = exact_manifest(f"{repository}@{top}", top)

        manifest = top
        if "manifests" in document:
            matches = [row["digest"] for row in document["manifests"] if platform(row.get("platform", {})) == PLATFORM]
            if len(matches) != 1 or not DIGEST.fullmatch(matches[0]):
                raise ValueError("published index needs exactly one linux/amd64 image")
            manifest = matches[0]
            document = exact_manifest(f"{repository}@{manifest}", manifest)

        config = document["config"]["digest"]
        layers = [row["digest"] for row in document["layers"]]
        if not layers or not all(DIGEST.fullmatch(value) for value in [config, *layers]):
            raise ValueError("invalid image descriptors")
        image = json.loads(inspect(f"{repository}@{manifest}", "--format", "{{json .Image}}"))
        if platform(image) != PLATFORM:
            raise ValueError("image config is not linux/amd64")
        if (image["config"].get("Labels") or {}).get(REVISION) != revision:
            raise ValueError("OCI revision disagrees with the release SHA")
        diff_ids = image["rootfs"]["diff_ids"]
        if len(diff_ids) != len(layers) or not all(DIGEST.fullmatch(value) for value in diff_ids):
            raise ValueError("image rootfs disagrees with layer descriptors")
        tag_now = json.loads(inspect(reference, "--format", "{{.Manifest.Digest | json}}"))
        if tag_now != top:
            raise ValueError("release tag changed during readback")
        return name, {"status": "verified", "tag_digest": top, "amd64_manifest_digest": manifest,
                      "config_digest": config, "layer_digests": layers, "rootfs_diff_ids": diff_ids,
                      "oci_revision": revision}
    except (KeyError, TypeError, ValueError, subprocess.TimeoutExpired) as error:
        if type(error) is ValueError:
            message = str(error)
        elif isinstance(error, subprocess.TimeoutExpired):
            message = "registry inspection timed out"
        else:
            message = "invalid registry response"
        return name, {"status": "incomplete", "reason": message}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("revision")
    parser.add_argument("ghcr")
    parser.add_argument("docker_hub")
    parser.add_argument("acr")
    args = parser.parse_args()
    repos = {"ghcr": args.ghcr, "docker_hub": args.docker_hub, "acr": args.acr}
    if not SHA.fullmatch(args.revision) or not all(REPO.fullmatch(repo) for repo in repos.values()):
        parser.error("provide an exact Git SHA and three registry repository paths")
    if len(set(repos.values())) != len(repos):
        parser.error("provide three distinct registry repository paths")
    with ThreadPoolExecutor(max_workers=3) as pool:
        results = dict(pool.map(lambda item: read_registry(args.revision, item), repos.items()))
    verified = all(row["status"] == "verified" for row in results.values())
    configs_match = verified and len({row["config_digest"] for row in results.values()}) == 1
    same_rootfs = verified and len({tuple(row["rootfs_diff_ids"]) for row in results.values()}) == 1
    print(json.dumps({"at_utc": datetime.now(timezone.utc).isoformat(), "read_only": True,
                      "revision": args.revision, "registries": results, "config_digest_matches": configs_match,
                      "rootfs_matches": same_rootfs, "verified": configs_match and same_rootfs}, indent=2))
    return 0 if configs_match and same_rootfs else 1


if __name__ == "__main__":
    raise SystemExit(main())
