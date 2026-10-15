# usage: gaps.py file.json [duration_s]  -- per-meter count and gaps (minutes) from
# stream-time stamps; duration defaults to 3954 s (the bench recording)
import sys, json, datetime as d
from collections import defaultdict
dur = float(sys.argv[2]) if len(sys.argv) > 2 else 3954
t = defaultdict(list)
for l in open(sys.argv[1]):
    try: m = json.loads(l)
    except Exception: continue
    ts = d.datetime.fromisoformat(m['Time'].replace('Z', '+00:00')).timestamp()
    t[m['Type']].append(ts)
for ty in ('SCM', 'SCM+', 'R900'):
    v = sorted(t.get(ty, []))
    edges = [0] + v + [dur]
    g = [(b - a) / 60 for a, b in zip(edges, edges[1:])]
    print(f"{ty:5} n={len(v):3} maxgap={max(g):5.1f}m  gaps(min)=" + ' '.join(f"{x:.1f}" for x in g[1:-1]))
