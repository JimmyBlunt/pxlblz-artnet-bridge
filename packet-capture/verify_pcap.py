#!/usr/bin/env python3
import json
import struct
import sys
from collections import Counter, defaultdict
from pathlib import Path

pcap_path = Path(sys.argv[1])
config_path = Path(sys.argv[2])
out_path = Path(sys.argv[3])

cfg = json.loads(config_path.read_text())
routes = [r for r in cfg["routes"] if r.get("enabled", False)]

expected_by_ip = defaultdict(set)
for r in routes:
    universes = (r["pixel_count"] + 169) // 170
    for u in range(r["universe_start"], r["universe_start"] + universes):
        expected_by_ip[r["target_ip"]].add(u)

def read_pcap(path):
    data = path.read_bytes()
    if len(data) < 24:
        raise AssertionError("pcap too short")
    magic = data[:4]
    if magic == b"\xd4\xc3\xb2\xa1":
        endian = "<"
    elif magic == b"\xa1\xb2\xc3\xd4":
        endian = ">"
    else:
        raise AssertionError("unsupported pcap magic " + magic.hex())
    off = 24
    while off + 16 <= len(data):
        ts_sec, ts_usec, incl_len, orig_len = struct.unpack_from(endian+"IIII", data, off)
        off += 16
        pkt = data[off:off+incl_len]
        off += incl_len
        yield ts_sec + ts_usec/1_000_000, pkt

def parse(pkt):
    ipoff = None
    for i in range(0, min(64, len(pkt)-20)):
        if pkt[i] >> 4 != 4:
            continue
        ihl = (pkt[i] & 0x0F) * 4
        if ihl < 20 or i + ihl + 8 > len(pkt):
            continue
        if pkt[i+9] != 17:
            continue
        total = struct.unpack_from("!H", pkt, i+2)[0]
        if total >= ihl + 8 and i + total <= len(pkt):
            ipoff = i
            break
    if ipoff is None:
        return None
    ihl = (pkt[ipoff] & 0x0F) * 4
    src = ".".join(str(x) for x in pkt[ipoff+12:ipoff+16])
    dst = ".".join(str(x) for x in pkt[ipoff+16:ipoff+20])
    udpoff = ipoff + ihl
    sport, dport, udplen, checksum = struct.unpack_from("!HHHH", pkt, udpoff)
    payload = pkt[udpoff+8:udpoff+udplen]
    if dport != 6454 or len(payload) < 18 or payload[:8] != b"Art-Net\x00":
        return None
    opcode = struct.unpack_from("<H", payload, 8)[0]
    if opcode != 0x5000:
        return None
    seq = payload[12]
    universe = struct.unpack_from("<H", payload, 14)[0] & 0x7FFF
    length = struct.unpack_from("!H", payload, 16)[0]
    art = payload[18:18+length]
    return dict(src=src,dst=dst,sport=sport,dport=dport,seq=seq,universe=universe,length=length,data=art)

packets = []
for ts, raw in read_pcap(pcap_path):
    p = parse(raw)
    if p:
        p["ts"] = ts
        packets.append(p)

if not packets:
    raise AssertionError("no ArtDmx packets captured")

allowed_ips = set(expected_by_ip)
seen_by_ip = defaultdict(set)
counts = Counter()
seq_groups = defaultdict(list)
odd_lengths = []
unexpected = []
bad_lengths = []
samples = {}

for p in packets:
    if p["dst"] not in allowed_ips:
        unexpected.append((p["dst"], p["universe"]))
        continue
    seen_by_ip[p["dst"]].add(p["universe"])
    counts[(p["dst"], p["universe"])] += 1
    seq_groups[p["seq"]].append(p)
    if p["length"] % 2:
        odd_lengths.append((p["dst"], p["universe"], p["length"]))
    if p["length"] < 2 or p["length"] > 512 or p["length"] != len(p["data"]):
        bad_lengths.append((p["dst"], p["universe"], p["length"], len(p["data"])))
    samples.setdefault((p["dst"], p["universe"]), p)

missing = {ip: sorted(expected_by_ip[ip] - seen_by_ip[ip]) for ip in expected_by_ip if expected_by_ip[ip] - seen_by_ip[ip]}
expected_keys = {(ip,u) for ip, us in expected_by_ip.items() for u in us}
expected_packet_count = len(expected_keys)

complete_sequences = []
for seq, ps in seq_groups.items():
    keys = {(p["dst"],p["universe"]) for p in ps}
    if keys == expected_keys and len(ps) == expected_packet_count:
        complete_sequences.append(seq)

if not complete_sequences:
    raise AssertionError("no complete 44-packet sequence in pcap")
if missing:
    raise AssertionError("missing universes: " + repr(missing))
if odd_lengths:
    raise AssertionError("odd ArtDmx lengths: " + repr(odd_lengths[:10]))
if bad_lengths:
    raise AssertionError("invalid ArtDmx lengths: " + repr(bad_lengths[:10]))
if unexpected:
    raise AssertionError("unexpected destinations/universes: " + repr(unexpected[:10]))

required_lengths = {
    ("10.0.0.253",121):100,
    ("10.0.0.253",126):174,
    ("10.0.0.253",132):90,
    ("10.0.0.253",137):390,
    ("10.0.0.253",141):36,
    ("10.0.0.253",145):300,
    ("10.0.0.253",149):6,
}
for key,want in required_lengths.items():
    got = samples[key]["length"]
    if got != want:
        raise AssertionError(f"{key} length={got} want={want}")

if ("10.0.0.253",138) in samples:
    raise AssertionError("BACK_PANEL U138 unexpectedly emitted")

summary = {
    "pcap": str(pcap_path),
    "captured_artdmx_packets": len(packets),
    "expected_packets_per_complete_frame": expected_packet_count,
    "complete_sequences": len(complete_sequences),
    "complete_sequence_values_sample": complete_sequences[:10],
    "destinations": {
        ip: {
            "expected_universes": sorted(expected_by_ip[ip]),
            "seen_universes": sorted(seen_by_ip[ip]),
            "packet_count": sum(v for (dip,u),v in counts.items() if dip == ip),
        }
        for ip in sorted(expected_by_ip)
    },
    "backpanel_tail_lengths": {f"{ip}:U{u}": samples[(ip,u)]["length"] for (ip,u) in required_lengths},
    "u138_absent": ("10.0.0.253",138) not in samples,
    "all_artdmx_lengths_even": not odd_lengths,
    "unexpected_packets": len(unexpected),
    "status": "PASS",
}
out_path.write_text(json.dumps(summary, indent=2) + "\n")
