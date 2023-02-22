// @ts-check
// One-time command socket. Sends a request, receives a response, and closes the socket.
import { base64DecodeString, base64EncodeString } from 'nomad-ui/utils/encode';

export const HEARTBEAT_INTERVAL = 10000; // ten seconds

export default class ExecSocketAction {
  constructor(socket, token) {
    this.socket = socket;
    this.token = token;

    socket.onopen = () => {
      console.log("SOCKET OPEN");
      this.sendWsHandshake();
      this.handleData('nomad status');
      // this.sendTtySize();
      // this.startHeartbeat();

      // terminal.onData((data) => {
      //   this.handleData(data);
      // });
    };

    socket.onmessage = (e) => {
      let json = JSON.parse(e.data);
      console.log('onMessage', e);

      // stderr messages will not be produced as the socket is opened with the tty flag
      if (json.stdout && json.stdout.data) {
        console.log('oh cool!!!', json.stdout, base64DecodeString(json.stdout.data));
        // terminal.write(base64DecodeString(json.stdout.data));
      }
    };

    socket.onclose = () => {
      this.stopHeartbeat();
      console.log('SOCKET CLOSED');
      // this.terminal.writeln('');
      // this.terminal.write(ANSI_UI_GRAY_400);
      // this.terminal.writeln('The connection has closed.');
      // Issue to add interpretation of close events: https://github.com/hashicorp/nomad/issues/7464
    };

    // terminal.resized = () => {
    //   this.sendTtySize();
    // };
  }

  // sendTtySize() {
  //   this.socket.send(
  //     JSON.stringify({
  //       tty_size: { width: this.terminal.cols, height: this.terminal.rows },
  //     })
  //   );
  // }

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
    console.log('send data', data);
    this.socket.send(
      JSON.stringify({ stdin: { data: base64EncodeString(data) } })
    );
    // Handle a carriage return / return key press
    console.log('send enter');
    this.socket.send(
      JSON.stringify({ stdin: { data: "DQ==" }})
    )
    // // Wait a second and then close the socket
    // setTimeout(() => {
    //   this.socket.send(
    //     JSON.stringify({ stdin: { close: true }})
    //   )
    // }, 3000);
    
  }
}
