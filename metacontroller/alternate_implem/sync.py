from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
from datadog import initialize, api
import hashlib
import json, os, sys

initialize() # <- dd libs

seen = {}

class Controller(BaseHTTPRequestHandler):
  def sync(self, obj, finalizing):
    mid = obj.get("metadata", {}).get("annotations", {}).get("monitor.datadoghq.com/id", None)

    if finalizing:
        res = api.Monitor.delete(mid)
        err = res.get("errors")
        if err and err != ["Monitor not found"]:
            return 500, err
        return 200, {"finalized": True}

    if mid:
        res = api.Monitor.update(mid, **obj.get("spec", {}))
        err = res.get("errors")
        if err == ["Monitor not found"]:
            mid = None
        elif err:
            return 500, err

    if not mid:
        mid = seen.get(self.hash(obj))

    if not mid:
        res = api.Monitor.create(**obj.get("spec", {}))
        mid = res.get("id")
        if not mid:
            return 500, res.get("errors")

    seen[self.hash(obj)] = mid
    return 200, {"status": {"id": int(mid)}, "annotations": {"monitor.datadoghq.com/id": str(mid)}}

  def do_POST(self):
    observed = json.loads(self.rfile.read(int(self.headers.getheader("content-length"))))
    status, msg= self.sync(observed["object"], observed["finalizing"])
    self.send_response(status)
    self.send_header("Content-type", "application/json")
    self.end_headers()
    self.wfile.write(json.dumps(msg))

  def hash(self, obj):
    return hashlib.md5(json.dumps(obj, sort_keys=True)).hexdigest()

if not os.getenv("DATADOG_API_KEY"):
    print("Please export DATADOG_API_KEY, DATADOG_APP_KEY, and DATADOG_HOST")
    sys.exit(1)

HTTPServer(("", 80), Controller).serve_forever()
