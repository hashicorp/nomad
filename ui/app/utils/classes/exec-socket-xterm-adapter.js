/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const ANSI_UI_GRAY_400 = '\x1b[38;2;142;150;163m';

import { base64DecodeString, base64EncodeString } from 'nomad-ui/utils/encode';

export const HEARTBEAT_INTERVAL = 10000; // ten seconds

export default class ExecSocketXtermAdapter {
  constructor(terminal, socket, token) {
    this.terminal = terminal;
    this.socket = socket;
    this.token = token;

    socket.onopen = () => {
      this.sendWsHandshake();
      this.sendTtySize();
      this.startHeartbeat();

      terminal.onData((data) => {
        this.handleData(data);
      });
    };

    socket.onmessage = (e) => {
      let json = JSON.parse(e.data);

      // stderr messages will not be produced as the socket is opened with the tty flag
      if (json.stdout && json.stdout.data) {
        terminal.write(base64DecodeString(json.stdout.data));
      }
    };

    socket.onclose = () => {
      this.stopHeartbeat();
      this.terminal.writeln('');
      this.terminal.write(ANSI_UI_GRAY_400);
      this.terminal.writeln('The connection has closed.');
      // Issue to add interpretation of close events: https://github.com/hashicorp/nomad/issues/7464
    };

    terminal.resized = () => {
      this.sendTtySize();
    };
  }

  sendTtySize() {
    this.socket.send(
      JSON.stringify({
        tty_size: { width: this.terminal.cols, height: this.terminal.rows },
      })
    );
  }

  sendWsHandshake() {
    this.socket.send(
      JSON.stringify({ version: 1, auth_token: this.token || '' })
    );
  }

  startHeartbeat() {
    this.heartbeatTimer = setInterval(() => {
      this.socket.send(JSON.stringify({}));
    }, HEARTBEAT_INTERVAL);
  }

  stopHeartbeat() {
    clearInterval(this.heartbeatTimer);
  }

  handleData(data) {
    this.socket.send(
      JSON.stringify({ stdin: { data: base64EncodeString(data) } })
    );
  }
}
