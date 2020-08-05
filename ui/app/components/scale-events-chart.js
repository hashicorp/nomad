import Component from '@ember/component';
import { computed } from '@ember/object';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class ScaleEventsChart extends Component {
  events = [];

  activeEvent = null;

  @computed('events.[]')
  get data() {
    return this.events.filterBy('hasCount').sortBy('time');
  }

  @computed('events.[]')
  get annotations() {
    return this.events.rejectBy('hasCount').map(ev => ({
      type: ev.error ? 'error' : 'info',
      time: ev.time,
      event: ev,
    }));
  }

  toggleEvent(ev) {
    if (this.activeEvent === ev) {
      this.closeEventDetails();
    } else {
      this.set('activeEvent', ev);
    }
  }

  closeEventDetails() {
    this.set('activeEvent', null);
  }
}
