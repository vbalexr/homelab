#!/usr/bin/env python3
"""Build the Coyote Den Modrinth pack (CoyoteDensPack-<version>.mrpack) served by the guide sidecar.

Runs locally, never on the server: the cluster only serves the finished file. It downloads the official "Satisfaction Guaranteed"
pack from Modrinth (exact mod versions, nothing converted), checks its hash, and changes only four things: the pack name, the
Simple Voice Chat mod (the server uses it), overrides/servers.dat, which puts the server in the multiplayer list, and icon.png
(../icon/pack-icon.png) at the pack root, which launchers may show as the instance icon (the format has no icon field).

Usage:  python3 build-mrpack.py
"""
import hashlib
import json
import pathlib
import struct
import urllib.parse
import urllib.request
import zipfile
import io

PROJECT = "satisfaction-guaranteed"
PACK_VERSION = "1.2.1"  # the official pack release to start from; bump on purpose, never follow "latest"
ICON = pathlib.Path(__file__).resolve().parent.parent / "icon" / "pack-icon.png"
PACK_NAME = "CoyoteDensPack"
SERVER_NAME = "Coyote Den"
SERVER_ADDRESS = "minecraft.vbalex.com"
VOICE_CHAT = "simple-voice-chat"
UA = {"User-Agent": "homelab-minecraft-guide"}
STAMP = (2026, 1, 1, 0, 0, 0)  # fixed timestamps: the zip only changes when its content does


def fetch(url):
    with urllib.request.urlopen(urllib.request.Request(url, headers=UA)) as r:
        return r.read()


def api(path, **params):
    return json.loads(fetch("https://api.modrinth.com/v2/" + path + "?" + urllib.parse.urlencode(params)))


def official_pack():
    versions = api(f"project/{PROJECT}/version")
    v = next((v for v in versions if v["version_number"] == PACK_VERSION), None)
    if not v:
        raise SystemExit(f"{PROJECT} {PACK_VERSION} not found on Modrinth")
    f = next((f for f in v["files"] if f["primary"]), v["files"][0])
    data = fetch(f["url"])
    if hashlib.sha512(data).hexdigest() != f["hashes"]["sha512"]:
        raise SystemExit("downloaded pack does not match the Modrinth hash")
    print(f"{PROJECT} {PACK_VERSION}: {f['filename']}")
    return data, v["game_versions"]


def voice_chat(mc):
    versions = api(f"project/{VOICE_CHAT}/version", loaders=json.dumps(["fabric"]), game_versions=json.dumps([mc]))
    if not versions:
        raise SystemExit(f"{VOICE_CHAT}: no Fabric build for {mc}")
    v = versions[0]
    f = next((f for f in v["files"] if f["primary"]), v["files"][0])
    print(f"{VOICE_CHAT}: {v['version_number']} ({v['version_type']})")
    return {
        "path": "mods/" + f["filename"],
        "hashes": {"sha1": f["hashes"]["sha1"], "sha512": f["hashes"]["sha512"]},
        "env": {"client": "required", "server": "unsupported"},
        "downloads": [f["url"]],
        "fileSize": f["size"],
    }


def nbt_str(x):
    b = x.encode()
    return struct.pack(">H", len(b)) + b


def servers_dat():
    # uncompressed NBT: {servers: [{name, ip}]}
    entry = b"\x08" + nbt_str("name") + nbt_str(SERVER_NAME) + b"\x08" + nbt_str("ip") + nbt_str(SERVER_ADDRESS) + b"\x00"
    return b"\x0a" + nbt_str("") + b"\x09" + nbt_str("servers") + b"\x0a" + struct.pack(">i", 1) + entry + b"\x00"


def main():
    data, _ = official_pack()
    with zipfile.ZipFile(io.BytesIO(data)) as src:
        names = src.namelist()
        if "overrides/servers.dat" in names or "icon.png" in names:
            raise SystemExit("the official pack already has overrides/servers.dat or icon.png, merge by hand")
        index = json.loads(src.read("modrinth.index.json"))
        others = {n: src.read(n) for n in names if n != "modrinth.index.json" and not n.endswith("/")}

    mc = index["dependencies"]["minecraft"]
    index["name"] = PACK_NAME
    index["summary"] = f"Satisfaction Guaranteed for the {SERVER_NAME} server ({SERVER_ADDRESS}), with Simple Voice Chat"
    mod = voice_chat(mc)
    if any(f["path"] == mod["path"] for f in index["files"]):
        raise SystemExit(f"{mod['path']} is already in the pack")
    index["files"].append(mod)

    out = f"{PACK_NAME}-{index['versionId']}.mrpack"
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr(zipfile.ZipInfo("modrinth.index.json", STAMP), json.dumps(index, indent=2))
        for n in sorted(others):
            z.writestr(zipfile.ZipInfo(n, STAMP), others[n], zipfile.ZIP_DEFLATED)
        z.writestr(zipfile.ZipInfo("overrides/servers.dat", STAMP), servers_dat(), zipfile.ZIP_DEFLATED)
        z.writestr(zipfile.ZipInfo("icon.png", STAMP), ICON.read_bytes(), zipfile.ZIP_DEFLATED)
    print(f"wrote {out}: {len(index['files'])} files (official {len(index['files']) - 1} + voice chat)")


if __name__ == "__main__":
    main()
