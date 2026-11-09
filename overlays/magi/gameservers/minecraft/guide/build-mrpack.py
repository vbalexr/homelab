#!/usr/bin/env python3
"""Build the VaCe Server Modrinth pack (VaCe-Server-<mc>.mrpack) served by the guide sidecar.

Resolves the newest Fabric build of each mod for MC_VERSION from the Modrinth API and writes a pack with
the mods (downloaded by the launcher, verified by hash) and an overrides/servers.dat that adds the server
to the multiplayer list. Run from this directory:  python3 build-mrpack.py
"""
import json
import struct
import urllib.parse
import urllib.request
import zipfile

MC_VERSION = "26.3"
PACK_VERSION = "1.0.0"
SERVER_NAME = "VaCe Server"
SERVER_ADDRESS = "minecraft.vbalex.com"
MODS = ["fabric-api", "sodium", "lithium", "ferrite-core", "entityculling", "immediatelyfast", "simple-voice-chat"]
UA = {"User-Agent": "homelab-minecraft-guide"}


def get(url):
    with urllib.request.urlopen(urllib.request.Request(url, headers=UA)) as r:
        return json.load(r)


def mod_file(slug):
    q = urllib.parse.urlencode({"loaders": '["fabric"]', "game_versions": f'["{MC_VERSION}"]'})
    versions = get(f"https://api.modrinth.com/v2/project/{slug}/version?{q}")
    if not versions:
        raise SystemExit(f"{slug}: no Fabric build for {MC_VERSION}")
    v = versions[0]
    f = next((f for f in v["files"] if f["primary"]), v["files"][0])
    print(f"{slug:20} {v['version_number']} ({v['version_type']})")
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
    loader = next(l["loader"]["version"] for l in get(f"https://meta.fabricmc.net/v2/versions/loader/{MC_VERSION}")
                  if l["loader"]["stable"])
    print(f"{'fabric-loader':20} {loader}")
    index = {
        "formatVersion": 1,
        "game": "minecraft",
        "versionId": PACK_VERSION,
        "name": SERVER_NAME,
        "summary": f"Performance mods and Simple Voice Chat for the {SERVER_NAME} ({SERVER_ADDRESS})",
        "files": [mod_file(m) for m in MODS],
        "dependencies": {"minecraft": MC_VERSION, "fabric-loader": loader},
    }
    out = f"VaCe-Server-{MC_VERSION}.mrpack"
    stamp = (2026, 1, 1, 0, 0, 0)  # fixed timestamps: the zip only changes when its content does
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr(zipfile.ZipInfo("modrinth.index.json", stamp), json.dumps(index, indent=2))
        z.writestr(zipfile.ZipInfo("overrides/servers.dat", stamp), servers_dat())
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
