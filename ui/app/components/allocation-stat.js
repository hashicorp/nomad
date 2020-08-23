import Component from '@ember/component';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class AllocationStat extends Component {
  allocation = null;
  statsTracker = null;
  isLoading = false;
  error = null;
  metric = 'memory'; // Either memory or cpu

  @computed('metric')
  get statClass() {
    return this.metric === 'cpu' ? 'is-info' : 'is-danger';
  }

  @alias('statsTracker.cpu.lastObject') cpu;
  @alias('statsTracker.memory.lastObject') memory;

  @computed('metric', 'cpu', 'memory')
  get stat() {
    const { metric } = this;
    if (metric === 'cpu' || metric === 'memory') {
      return this[this.metric];
    }

    return undefined;
  }

  @computed('metric', 'stat.used')
  get formattedStat() {
    if (!this.stat) return undefined;
    if (this.metric === 'memory') return formatBytes([this.stat.used]);
    return this.stat.used;
  }

  @computed('metric', 'statsTracker.{reservedMemory,reservedCPU}')
  get formattedReserved() {
    if (this.metric === 'memory') return `${this.statsTracker.reservedMemory} MiB`;
    if (this.metric === 'cpu') return `${this.statsTracker.reservedCPU} MHz`;
    return undefined;
  }
}
