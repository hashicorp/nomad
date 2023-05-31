// @ts-check
import Component from '@glimmer/component';
// import sumAggregation from '../utils/properties/sum-aggregation';

export default class VersionedAllocationStatusBarComponent extends Component {
  /**
   * Goes over the job's versionedSummary's statuses and searches for the highest key.
   */
  get latestVersion() {
    // I am so sorry
    const awfulButWorks = Object.values(this.args.job.versionedSummary).map((g) => Object.values(g)).flat().map((status) => Object.keys(status || {})).flat().map((version) => parseInt(version)).uniq().sort().reverse().objectAt(0);
    return awfulButWorks;
  }
  get data() {
    const allocationsAtLatestVersion = Object.values(this.args.job.versionedSummary).flat();
    // if (this.args.job.name === "fails_every_10") {
    //   console.log('investigating:');
    //   console.log(this.latestVersion, this.args.job.versionedSummary, Object.values(this.args.job.versionedSummary));
    //   console.log('ahumm', Object.values(this.args.job.versionedSummary).mapBy('Starting').map((status) => (status || {})[this.latestVersion.toString()]));
    //   console.log(Object.values(this.args.job.versionedSummary).mapBy('Starting').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+n) || 0);
    // }
    return [
      {
        label: 'Queued',
        value: Object.values(this.args.job.versionedSummary).mapBy('Queued').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+(n||0)) || 0,
        className: 'queued',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Starting',
        value: Object.values(this.args.job.versionedSummary).mapBy('Starting').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+(n||0)) || 0,
        className: 'starting',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Running',
        value: Object.values(this.args.job.versionedSummary).mapBy('Running').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+(n||0)) || 0,
        className: 'running',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Complete',
        value: Object.values(this.args.job.versionedSummary).mapBy('Complete').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+(n||0)) || 0,
        className: 'complete',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Failed',
        value: Object.values(this.args.job.versionedSummary).mapBy('Failed').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+(n||0)) || 0,
        className: 'failed',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Lost',
        value: Object.values(this.args.job.versionedSummary).mapBy('Lost').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+(n||0)) || 0,
        className: 'lost',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
     
    ]
    return [];
  }

  get runningCount() {
    return this.data.findBy('label', 'Running').value;
  }

  // TODO: make this line up more closely with what we do within steady, maybe?
  // First, find out if taskgroups[].count can be gotten from list level.
  get expectedCount() {
    return this.data.mapBy('value').reduce((m,n) => m + n);
  }
}
