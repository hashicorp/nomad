// @ts-check
import Component from '@glimmer/component';
// import sumAggregation from '../utils/properties/sum-aggregation';

export default class VersionedAllocationStatusBarComponent extends Component {
  /**
   * Goes over the job's versionedSummary's statuses and searches for the highest key.
   */
  get latestVersion() {
    return Object.values(this.args.job.versionedSummary)
      .mapBy('Version')
      .sort()
      .reverse()
      .objectAt(0);
  }
  get data() {
    const allocationsAtLatestVersion =
      this.args.job.versionedSummary[this.latestVersion.toString()];
    console.log('allocstatusesAtLatestVersion', allocationsAtLatestVersion);
    // Just the running ones,
    const runningAllocations = Object.values(allocationsAtLatestVersion.Groups)
      .mapBy('Running')
      .reduce((m, n) => m + n);
    console.log('sorunning', runningAllocations);
    // if (this.args.job.name === "fails_every_10") {
    //   console.log('investigating:');
    //   console.log(this.latestVersion, this.args.job.versionedSummary, Object.values(this.args.job.versionedSummary));
    //   console.log('ahumm', Object.values(this.args.job.versionedSummary).mapBy('Starting').map((status) => (status || {})[this.latestVersion.toString()]));
    //   console.log(Object.values(this.args.job.versionedSummary).mapBy('Starting').map((status) => (status || {})[this.latestVersion.toString()]).reduce((m,n) => (m||0)+n) || 0);
    // }
    return [
      {
        label: 'Queued',
        value: Object.values(allocationsAtLatestVersion.Groups)
          .mapBy('Queued')
          .reduce((m, n) => m + n),
        className: 'queued',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Starting',
        value: Object.values(allocationsAtLatestVersion.Groups)
          .mapBy('Starting')
          .reduce((m, n) => m + n),
        className: 'starting',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Running',
        value: Object.values(allocationsAtLatestVersion.Groups)
          .mapBy('Running')
          .reduce((m, n) => m + n),
        className: 'running',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Complete',
        value: Object.values(allocationsAtLatestVersion.Groups)
          .mapBy('Complete')
          .reduce((m, n) => m + n),
        className: 'complete',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Failed',
        value: Object.values(allocationsAtLatestVersion.Groups)
          .mapBy('Failed')
          .reduce((m, n) => m + n),
        className: 'failed',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
      {
        label: 'Lost',
        value: Object.values(allocationsAtLatestVersion.Groups)
          .mapBy('Lost')
          .reduce((m, n) => m + n),
        className: 'lost',
        // legendLink: this.generateLegendLink(this.job, 'queued'),
      },
    ];
    return [];
  }

  get runningCount() {
    return this.data.findBy('label', 'Running').value;
  }

  // TODO: make this line up more closely with what we do within steady, maybe?
  // First, find out if taskgroups[].count can be gotten from list level.
  get expectedCount() {
    return this.data.mapBy('value').reduce((m, n) => m + n);
  }
}
