from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
from datadog import initialize, api
import json, os, sys

initialize() # <- dd libs

class Controller(BaseHTTPRequestHandler):
  def sync(self, parent, finalizing):
    mid = parent.get("status", {}).get("id", None)

    if finalizing:
        api.Monitor.delete(mid)
        return {"finalized": True}

    if mid:
        res = api.Monitor.update(mid, **parent.get("spec", {}))
        if res.get("errors", {}) == ["Monitor not found"]:
            mid = None

    if not mid:
        res = api.Monitor.create(**parent.get("spec", {}))
        mid = res.get("id")

    return {"status": {"id": mid}}

  def do_POST(self):
    observed = json.loads(self.rfile.read(int(self.headers.getheader("content-length"))))
    desired = self.sync(observed["parent"], observed["finalizing"])

    self.send_response(200)
    self.send_header("Content-type", "application/json")
    self.end_headers()
    self.wfile.write(json.dumps(desired))

if not os.getenv("DATADOG_API_KEY"):
    print("Please export DATADOG_API_KEY, DATADOG_APP_KEY, and DATADOG_HOST")
    sys.exit(1)

HTTPServer(("", 80), Controller).serve_forever()
