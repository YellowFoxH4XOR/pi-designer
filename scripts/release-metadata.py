#!/usr/bin/env python3
"""Validated release metadata shared by local builds, packaging and CI."""
import base64
import os
import plistlib
import re
import sys
from pathlib import Path
from urllib.parse import urlsplit


def version_parts(version):
    if not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", version):
        raise ValueError("Use a stable major.minor.patch version, e.g. 0.3.0 (no prerelease suffix).")
    return tuple(map(int, version.split(".")))


def feed_url(repository):
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+", repository):
        raise ValueError("Release repository must be owner/name.")
    if repository.split("/")[1] in (".", ".."):
        raise ValueError("Invalid release repository name.")
    return f"https://github.com/{repository}/releases/latest/download/appcast.xml"


def configure_plist(filename, env):
    filename = Path(filename)
    with filename.open("rb") as source:
        info = plistlib.load(source)
    version = env.get("PIDESIGNER_VERSION", info["CFBundleShortVersionString"])
    version_parts(version)
    info["CFBundleShortVersionString"] = version
    info["CFBundleVersion"] = version
    public_key = env.get("SPARKLE_PUBLIC_KEY", "").strip()
    feed = env.get("PIDESIGNER_UPDATE_FEED_URL", "").strip()
    if bool(public_key) != bool(feed):
        raise ValueError("Supply both SPARKLE_PUBLIC_KEY and PIDESIGNER_UPDATE_FEED_URL, or neither for a local build.")
    if env.get("PIDESIGNER_RELEASE_BUILD") == "1" and not public_key:
        raise ValueError("Release builds require a signed update feed and public key.")
    if public_key:
        if len(base64.b64decode(public_key, validate=True)) != 32:
            raise ValueError("Sparkle public key must contain 32 base64-encoded bytes.")
        url = urlsplit(feed)
        if url.scheme != "https" or not url.hostname or url.username or url.password or url.fragment:
            raise ValueError("Update feed must be an HTTPS URL without credentials or a fragment.")
        info["SUPublicEDKey"] = public_key
        info["SUFeedURL"] = feed
    else:
        info.pop("SUPublicEDKey", None)
        info.pop("SUFeedURL", None)
    # Defaults only: Sparkle persists user changes itself. Never reset them at launch.
    info["SUEnableAutomaticChecks"] = True
    info["SUAutomaticallyUpdate"] = False
    info["SUEnableSystemProfiling"] = False
    info["SUVerifyUpdateBeforeExtraction"] = True
    info["SURequireSignedFeed"] = True
    with filename.open("wb") as destination:
        plistlib.dump(info, destination, sort_keys=False)


def main():
    command = sys.argv[1]
    if command == "configure-plist":
        configure_plist(sys.argv[2], os.environ)
    elif command == "feed-url":
        print(feed_url(sys.argv[2]))
    elif command == "release-info":
        tag = os.environ["GITHUB_REF_NAME"]
        if not tag.startswith("v"):
            raise ValueError("Release ref must be a v-prefixed version tag.")
        version = tag[1:]
        version_parts(version)
        repository = os.environ.get("RELEASE_REPOSITORY") or os.environ["GITHUB_REPOSITORY"]
        print(f"version={version}\ntag={tag}\nrepository={repository}\nfeed={feed_url(repository)}")
    elif command == "check-newer":
        new = version_parts(sys.argv[2])
        old = sys.argv[3].removeprefix("v")
        if old and new <= version_parts(old):
            raise ValueError("New release version must be greater than the latest published version.")
    else:
        raise ValueError(f"Unknown command: {command}")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, IndexError) as error:
        sys.exit(str(error))
