import { computed, set } from '@ember/object';
import DistributionBar from './distribution-bar';
import classic from 'ember-classic-decorator';

@classic
export default class ClientStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  'data-test-client-status-bar' = true;
  jobClientStatus = null;

  // Provide an action with access to the router
  gotoClient() {}

  didRender() {
    // append data-test attribute to test link to pre-filtered client tab view
    this.element.querySelectorAll('.bars > g').forEach(g => {
      g.setAttribute(`data-test-client-status-${g.className.baseVal}`, g.className.baseVal);
    });

    // append on click event to bar chart
    const { _data, chart } = this;
    const filteredData = _data.filter(d => d.value > 0);
    filteredData.forEach((d, index) => {
      set(d, 'index', index);
    });
    chart
      .select('.bars')
      .selectAll('g')
      .data(filteredData, d => d.label)
      .on('click', d => {
        let label = d.label === 'Not Scheduled' ? 'notScheduled' : d.label;
        this.gotoClient(label);
      });
  }

  @computed('jobClientStatus.byStatus')
  get data() {
    const {
      queued,
      starting,
      running,
      complete,
      degraded,
      failed,
      lost,
      notScheduled,
    } = this.jobClientStatus.byStatus;
    return [
      {
        label: 'Queued',
        value: queued.length,
        className: 'queued',
      },
      {
        label: 'Starting',
        value: starting.length,
        className: 'starting',
        layers: 2,
      },
      {
        label: 'Running',
        value: running.length,
        className: 'running',
      },
      {
        label: 'Complete',
        value: complete.length,
        className: 'complete',
      },
      {
        label: 'Degraded',
        value: degraded.length,
        className: 'degraded',
      },
      {
        label: 'Failed',
        value: failed.length,
        className: 'failed',
      },
      {
        label: 'Lost',
        value: lost.length,
        className: 'lost',
      },
      {
        label: 'Not Scheduled',
        value: notScheduled.length,
        className: 'not-scheduled',
      },
    ];
  }
}
