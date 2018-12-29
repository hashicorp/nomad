import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';

export default DistributionBar.extend({
  layoutName: 'components/distribution-bar',

  job: null,

  'data-test-children-status-bar': true,

  data: computed('job.{pendingChildren,runningChildren,deadChildren}', function() {
    if (!this.get('job')) {
      return [];
    }

    const children = this.get('job').getProperties(
      'pendingChildren',
      'runningChildren',
      'deadChildren'
    );
    return [
      { label: 'Pending', value: children.pendingChildren, className: 'queued' },
      { label: 'Running', value: children.runningChildren, className: 'running' },
      { label: 'Dead', value: children.deadChildren, className: 'complete' },
    ];
  }),
});
