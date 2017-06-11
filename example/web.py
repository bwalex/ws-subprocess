#!/usr/bin/env python
from flask import Flask, redirect, url_for, json, Response, request
import sys

app = Flask(__name__)

@app.route("/")
def index():
    return redirect(url_for('static', filename='index.html'))

@app.route("/ws-controller")
def wsreq():
    token = request.args.get('token')
    if not token or len(token) == 0:
        return "", 400

    app.logger.info('ws-subprocess controller request with token=%s', token)

    if token != 'abcd':
        return "", 403

    command_response = {
        'command': app.config['WSS_COMMAND'],
        'args': app.config['WSS_ARGS']
    }

    return Response(json.dumps(command_response), mimetype='application/json')

if __name__ == "__main__":
    if len(sys.argv) < 2:
        sys.exit(1)

    app.config['WSS_COMMAND'] = sys.argv[1]
    app.config['WSS_ARGS'] = sys.argv[2:]
    app.run()
