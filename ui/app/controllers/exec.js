import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { Terminal } from 'xterm';

export default Controller.extend({
  system: service(),

  init() {
    this._super(...arguments);

    this.terminal = new Terminal();
    window.execTerminal = this.terminal; // FIXME tragique, for acceptance tests…?

    this.terminal.write('Select a task to start your session.');
  },
});
