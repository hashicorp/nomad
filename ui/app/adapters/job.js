import { inject as service } from '@ember/service';
import Watchable from './watchable';

export default Watchable.extend({
  system: service(),

  findAll() {
    const namespace = this.get('system.activeNamespace');
    return this._super(...arguments).then(data => {
      data.forEach(job => {
        job.Namespace = namespace ? namespace.get('id') : 'default';
      });
      return data;
    });
  },

  findRecord(store, type, id, snapshot) {
    const [, namespace] = JSON.parse(id);
    const namespaceQuery = namespace && namespace !== 'default' ? { namespace } : {};

    return this._super(store, type, id, snapshot, namespaceQuery);
  },

  urlForFindAll() {
    const url = this._super(...arguments);
    const namespace = this.get('system.activeNamespace.id');
    return associateNamespace(url, namespace);
  },

  urlForFindRecord(id, type, hash) {
    const [name, namespace] = JSON.parse(id);
    let url = this._super(name, type, hash);
    return associateNamespace(url, namespace);
  },

  urlForUpdateRecord(id, type, hash) {
    const [name, namespace] = JSON.parse(id);
    let url = this._super(name, type, hash);
    return associateNamespace(url, namespace);
  },

  xhrKey(url, method, options = {}) {
    const plainKey = this._super(...arguments);
    const namespace = options.data && options.data.namespace;
    return associateNamespace(plainKey, namespace);
  },

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
    const jobId = job.get('id');
    const store = this.get('store');
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
    return this.ajax(this.urlForUpdateRecord(job.get('id'), 'job'), 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
      },
    });
  },
});

function associateNamespace(url, namespace) {
  if (namespace && namespace !== 'default') {
    url += `?namespace=${namespace}`;
  }
  return url;
}

function addToPath(url, extension = '') {
  const [path, params] = url.split('?');
  let newUrl = `${path}${extension}`;

  if (params) {
    newUrl += `?${params}`;
  }

  return newUrl;
}
