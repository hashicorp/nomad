// @ts-check
// One-time command socket. Sends a request, receives a response, and closes the socket.
import { base64DecodeString, base64EncodeString } from 'nomad-ui/utils/encode';
import ActionModel from '../../models/action';

export const HEARTBEAT_INTERVAL = 10000; // ten seconds

export default class ExecSocketAction {
  // @service flashMessages;
  /**
   * 
   * @param {*} socket 
   * @param {*} token 
   * @param {ActionModel} action 
   */
  constructor(socket, token, action) {
    this.socket = socket;
    this.token = token;
    action.messageBuffer = "";

    socket.onopen = () => {
      this.sendWsHandshake();
    };

    socket.onmessage = (e) => {
      let json = JSON.parse(e.data);
      if (json.stdout && json.stdout.data) {
        action.messageBuffer += base64DecodeString(json.stdout.data);
        action.messageBuffer += "\n";
      }
    };

  }

  sendWsHandshake() {
    this.socket.send(
      JSON.stringify({ version: 1, auth_token: this.token || '' })
    );
  }

}
