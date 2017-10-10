import Ember from 'ember';
import { HOSTS } from './common';

const { copy } = Ember;

export function findLeader(schema) {
  const agent = schema.agents.first();
  return `${agent.address}:${agent.tags.port}`;
}

export default function() {
  this.timing = 0; // delay for each request, automatically set to 0 during testing

  this.namespace = 'v1';

  this.get('/jobs', function({ jobs }, { queryParams }) {
    const json = this.serialize(jobs.all());
    return json
      .filter(job => (queryParams.namespace ? job.NamespaceID === queryParams.namespace : true))
      .map(job => filterKeys(job, 'TaskGroups', 'NamespaceID'));
  });

  this.get('/job/:id');

  this.get('/job/:id/summary', function({ jobSummaries }, { params }) {
    return this.serialize(jobSummaries.findBy({ jobId: params.id }));
  });

  this.get('/job/:id/allocations', function({ allocations }, { params }) {
    return this.serialize(allocations.where({ jobId: params.id }));
  });

  this.get('/job/:id/versions', function({ jobVersions }, { params }) {
    return this.serialize(jobVersions.where({ jobId: params.id }));
  });

  this.get('/job/:id/deployments', function({ deployments }, { params }) {
    return this.serialize(deployments.where({ jobId: params.id }));
  });

  this.get('/deployment/:id');

  this.get('/deployment/allocations/:id', function(schema, { params }) {
    const job = schema.jobs.find(schema.deployments.find(params.id).jobId);
    const allocations = schema.allocations.where({ jobId: job.id });

    return this.serialize(allocations.slice(0, 3));
  });

  this.get('/nodes', function({ nodes }) {
    const json = this.serialize(nodes.all());
    return json;
  });

  this.get('/node/:id');

  this.get('/node/:id/allocations', function({ allocations }, { params }) {
    return this.serialize(allocations.where({ nodeId: params.id }));
  });

  this.get('/allocation/:id');

  this.get('/namespaces');
  this.get('/namespace/:id');

  this.get('/agent/members', function({ agents }) {
    return {
      Members: this.serialize(agents.all()),
    };
  });

  this.get('/status/leader', function(schema) {
    return JSON.stringify(findLeader(schema));
  });

  // TODO: in the future, this hack may be replaceable with dynamic host name
  // support in pretender: https://github.com/pretenderjs/pretender/issues/210
  HOSTS.forEach(host => {
    this.get(`http://${host}/v1/client/allocation/:id/stats`, function(
      { clientAllocationStats },
      { params }
    ) {
      return this.serialize(clientAllocationStats.find(params.id));
    });

    this.get(`http://${host}/v1/client/stats`, function({ clientStats }) {
      return this.serialize(clientStats.find(host));
    });
  });
}

function filterKeys(object, ...keys) {
  const clone = copy(object, true);

  keys.forEach(key => {
    delete clone[key];
  });

  return clone;
}
