import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';

const LEVELS = ['error', 'warn', 'info', 'debug', 'trace'].map(level => ({
  value: level,
  label: level.capitalize(),
}));

@tagName('')
export default class AgentMonitor extends Component {
  levels = LEVELS;
  client = null;
  server = null;
  onLevelChange() {}

  // Public string interface
  level = null;

  // Private object interface (value, label)
  _level = null;

  init() {
    super.init(...arguments);
    this.set('_level', LEVELS.findBy('value', 'info'));
  }

  didReceiveAttrs() {
    this.set('_level', LEVELS.findBy('value', this.level));
  }

  setLevel(level) {
    this.set('_level', level);
    this.onLevelChange(level);
  }
}
