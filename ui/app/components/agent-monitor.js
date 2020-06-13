import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import { tagName } from '@ember-decorators/component';
import { logger } from 'nomad-ui/utils/classes/log';

const LEVELS = ['error', 'warn', 'info', 'debug', 'trace'].map(level => ({
  value: level,
  label: level.capitalize(),
}));

@tagName('')
export default class AgentMonitor extends Component {
  @service token;

  levels = LEVELS;
  client = null;
  server = null;
  onLevelChange() {}

  // Public string interface
  level = null;

  // Private object interface (value, label)
  _level = null;

  monitorUrl = '/v1/agent/monitor';
  isStreaming = true;

  init() {
    super.init(...arguments);
    this.set('_level', LEVELS.findBy('value', 'info'));
  }

  didReceiveAttrs() {
    this.set('_level', LEVELS.findBy('value', this.level));
  }

  @computed('level', 'client.id', 'server.id')
  get monitorParams() {
    assert(
      'Provide a client OR a server to AgentMonitor, not both.',
      this.server != null || this.client != null
    );

    const type = this.server ? 'server_id' : 'client_id';
    const id = this.server ? this.server.id : this.client && this.client.id;

    return {
      log_level: this.level,
      [type]: id,
    };
  }

  @logger('monitorUrl', 'monitorParams', function logFetch() {
    return url =>
      this.token.authorizedRequest(url).then(response => {
        return response;
      });
  })
  logger;

  setLevel(level) {
    this.logger.stop();
    this.set('_level', level);
    this.onLevelChange(level);
  }

  toggleStream() {
    this.set('streamMode', 'streaming');
    this.toggleProperty('isStreaming');
  }
}
