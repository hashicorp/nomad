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

  @computed('jobClientStatus')
  get data() {
    return [
      {
        label: 'Queued',
        value: this.jobClientStatus.byStatus.queued.length,
        className: 'queued',
      },
      {
        label: 'Starting',
        value: this.jobClientStatus.byStatus.starting.length,
        className: 'starting',
        layers: 2,
      },
      {
        label: 'Running',
        value: this.jobClientStatus.byStatus.running.length,
        className: 'running',
      },
      {
        label: 'Complete',
        value: this.jobClientStatus.byStatus.complete.length,
        className: 'complete',
      },
      {
        label: 'Degraded',
        value: this.jobClientStatus.byStatus.degraded.length,
        className: 'degraded',
      },
      {
        label: 'Failed',
        value: this.jobClientStatus.byStatus.failed.length,
        className: 'failed',
      },
      {
        label: 'Lost',
        value: this.jobClientStatus.byStatus.lost.length,
        className: 'lost',
      },
      {
        label: 'Not Scheduled',
        value: this.jobClientStatus.byStatus.notScheduled.length,
        className: 'not-scheduled',
      },
    ];
  }
}
