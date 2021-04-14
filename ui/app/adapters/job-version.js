import ApplicationAdapter from './application';
import addToPath from 'nomad-ui/utils/add-to-path';

export default class JobVersionAdapter extends ApplicationAdapter {
  revertTo(jobVersion) {
    const jobAdapter = this.store.adapterFor('job');

    const url = addToPath(jobAdapter.urlForFindRecord(jobVersion.get('job.id'), 'job'), '/revert');
    const [jobName] = JSON.parse(jobVersion.get('job.id'));

    return this.ajax(url, 'POST', {
      data: {
        JobID: jobName,
        JobVersion: jobVersion.number,
      },
    });
  }
}
