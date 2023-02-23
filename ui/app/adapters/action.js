// @ts-check
import { default as ApplicationAdapter, namespace } from './application';
import ActionModel from '../models/action';
import { inject as service } from '@ember/service';
import ExecSocketAction from 'nomad-ui/utils/classes/exec-socket-action';

export default class ActionAdapter extends ApplicationAdapter {
  @service sockets;
  @service token;
  @service flashMessages;

  /**
   * @param {ActionModel} action
   * @returns
  */
  perform(action) {
    const { name, args, command } = action;
    let allocation = action.job.get('allocations').objectAt(0); // TODO: HACK ALERT
    let task = allocation.get('states').objectAt(0); //TODO: HACK ALERT
    let socket = this.sockets.getTaskStateSocket(task, null, name);
    let adapter = new ExecSocketAction(socket, this.token.secret, action);
    adapter.socket.onerror = (evt) => {
      console.log("FUUUUUUUCK", evt);
    }
    adapter.socket.onclose = (evt) => {
      console.log('EVT', evt);
      console.log('abacus',evt.target);
      console.log('mesbuf', action.messageBuffer)
      this.flashMessages.add({
        title: 'Action Performed',
        message: action.messageBuffer,
        code: true,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
      action.messageBuffer = "";
    };
  }

  // openAndConnectSocket(command) {
  //   if (this.taskState) {
  //     this.set('socketOpen', true);
  //     this.set('command', command);
  //     this.socket = this.sockets.getTaskStateSocket(this.taskState, command);

  //     new ExecSocketAction(this.socket, this.token.secret);
  //   } else {
  //     this.terminal.writeln(
  //       `Failed to open a socket because task ${this.taskName} is not active.`
  //     );
  //   }
  // }

  
}
