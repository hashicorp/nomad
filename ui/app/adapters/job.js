import Watchable from './watchable';
import addToPath from 'nomad-ui/utils/add-to-path';
import WithNamespaceIDs from 'nomad-ui/mixins/with-namespace-ids';

export default Watchable.extend(WithNamespaceIDs, {
  relationshipFallbackLinks: {
    summary: '/summary',
  },

  fetchRawDefinition(job) {
    const url = this.urlForFindRecord(job.get('id'), 'job');
    return this.ajax(url, 'GET');
  },

  forcePeriodic(job) {
    if (job.get('periodic')) {
      const url = addToPath(this.urlForFindRecord(job.get('id'), 'job'), '/periodic/force');
      return this.ajax(url, 'POST');
    }
  },

  stop(job) {
    const url = this.urlForFindRecord(job.get('id'), 'job');
    return this.ajax(url, 'DELETE');
  },

  parse(spec) {
    const url = addToPath(this.urlForFindAll('job'), '/parse');
    return this.ajax(url, 'POST', {
      data: {
        JobHCL: spec,
        Canonicalize: true,
      },
    });
  },

  plan(job) {
    const jobId = job.get('id') || job.get('_idBeforeSaving');
    const store = this.store;
    const url = addToPath(this.urlForFindRecord(jobId, 'job'), '/plan');

    return this.ajax(url, 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
        Diff: true,
      },
    }).then(json => {
      json.ID = jobId;
      store.pushPayload('job-plan', { jobPlans: [json] });
      return store.peekRecord('job-plan', jobId);
    });
  },

  // Running a job doesn't follow REST create semantics so it's easier to
  // treat it as an action.
  run(job) {
    return this.ajax(this.urlForCreateRecord('job'), 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
      },
    });
  },

  update(job) {
    const jobId = job.get('id') || job.get('_idBeforeSaving');
    return this.ajax(this.urlForUpdateRecord(jobId, 'job'), 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
      },
    });
  },
});
