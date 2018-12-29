import Ember from 'ember';
import Response from 'ember-cli-mirage/response';
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

  this.get('/namespaces', function({ namespaces }) {
    const records = namespaces.all();

    if (records.length) {
      return this.serialize(records);
    }

    return new Response(501, {}, null);
  });

  this.get('/namespace/:id', function({ namespaces }, { params }) {
    if (namespaces.all().length) {
      return this.serialize(namespaces.find(params.id));
    }

    return new Response(501, {}, null);
  });

  this.get('/agent/members', function({ agents }) {
    return {
      Members: this.serialize(agents.all()),
    };
  });

  this.get('/status/leader', function(schema) {
    return JSON.stringify(findLeader(schema));
  });

  this.get('/acl/token/self', function({ tokens }, req) {
    const secret = req.requestHeaders['X-Nomad-Token'];
    const tokenForSecret = tokens.findBy({ secretId: secret });

    // Return the token if it exists
    if (tokenForSecret) {
      return this.serialize(tokenForSecret);
    }

    // Client error if it doesn't
    return new Response(400, {}, null);
  });

  this.get('/acl/token/:id', function({ tokens }, req) {
    const token = tokens.find(req.params.id);
    const secret = req.requestHeaders['X-Nomad-Token'];
    const tokenForSecret = tokens.findBy({ secretId: secret });

    // Return the token only if the request header matches the token
    // or the token is of type management
    if (token.secretId === secret || (tokenForSecret && tokenForSecret.type === 'management')) {
      return this.serialize(token);
    }

    // Return not authorized otherwise
    return new Response(403, {}, null);
  });

  this.get('/acl/policy/:id', function({ policies, tokens }, req) {
    const policy = policies.find(req.params.id);
    const secret = req.requestHeaders['X-Nomad-Token'];
    const tokenForSecret = tokens.findBy({ secretId: secret });

    // Return the policy only if the token that matches the request header
    // includes the policy or if the token that matches the request header
    // is of type management
    if (
      tokenForSecret &&
      (tokenForSecret.policies.includes(policy) || tokenForSecret.type === 'management')
    ) {
      return this.serialize(policy);
    }

    // Return not authorized otherwise
    return new Response(403, {}, null);
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
