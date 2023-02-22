import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class JobActionComponent extends Component {
  // @service sockets;
  // openAndConnectSocket(command) {
  //   if (this.taskState) {
  //     this.set('socketOpen', true);
  //     this.set('command', command);
  //     this.socket = this.sockets.getTaskStateSocket(this.taskState, command);

  //     new ExecSocketXtermAdapter(this.terminal, this.socket, this.token.secret);
  //   } else {
  //     this.terminal.writeln(
  //       `Failed to open a socket because task ${this.taskName} is not active.`
  //     );
  //   }
  // }

}
