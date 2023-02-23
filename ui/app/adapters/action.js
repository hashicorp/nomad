// @ts-check
import { default as ApplicationAdapter, namespace } from './application';
import ActionModel from '../models/action';
import { inject as service } from '@ember/service';
import ExecSocketAction from 'nomad-ui/utils/classes/exec-socket-action';

export default class ActionAdapter extends ApplicationAdapter {
  @service sockets;
  @service token;

  /**
   * @param {ActionModel} action
   * @returns
  */
  perform(action) {
    const { name, args, command } = action;
    let allocation = action.job.get('allocations').objectAt(0); // TODO: HACK ALERT
    console.log('action adapter perform', name);
    // const url = `/${this.namespace}/client/allocation/${allocation.id}/exec`;
    let task = allocation.get('states').objectAt(0); //TODO: HACK ALERT
    let socket = this.sockets.getTaskStateSocket(task, null, name);
    console.log('socket?', socket, this.token.secret);
    let adapter = new ExecSocketAction(socket, this.token.secret);
    // console.log('adapter', adapter);
    // TODO: HACKY WAITER
    // setTimeout(() => {
    //   adapter.handleAction(name);
    // }, 500);

    // console.log('url', url);
    // return this.ajax(url, 'POST', {
    //   data: {
    //     name,
    //     args: JSON.stringify(args),
    //     command        
    //   },
    // });
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
