import Ember from 'ember';
import Response from 'ember-cli-mirage/response';
import { HOSTS } from './common';
import { logFrames, logEncode } from './data/logs';
import { generateDiff } from './factories/job-version';
import { generateTaskGroupFailures } from './factories/evaluation';

const { copy } = Ember;

export function findLeader(schema) {
  const agent = schema.agents.first();
  return `${agent.address}:${agent.tags.port}`;
}

export default function() {
  this.timing = 0; // delay for each request, automatically set to 0 during testing

  this.namespace = 'v1';
  this.trackRequests = Ember.testing;

  const nomadIndices = {}; // used for tracking blocking queries
  const server = this;
  const withBlockingSupport = function(fn) {
    return function(schema, request) {
      // Get the original response
      let { url } = request;
      url = url.replace(/index=\d+[&;]?/, '');
      const response = fn.apply(this, arguments);

      // Get and increment the appropriate index
      nomadIndices[url] || (nomadIndices[url] = 2);
      const index = nomadIndices[url];
      nomadIndices[url]++;

      // Annotate the response with the index
      if (response instanceof Response) {
        response.headers['X-Nomad-Index'] = index;
        return response;
      }
      return new Response(200, { 'x-nomad-index': index }, response);
    };
  };

  this.get(
    '/jobs',
    withBlockingSupport(function({ jobs }, { queryParams }) {
      const json = this.serialize(jobs.all());
      const namespace = queryParams.namespace || 'default';
      return json
        .filter(
          job =>
            namespace === 'default'
              ? !job.NamespaceID || job.NamespaceID === namespace
              : job.NamespaceID === namespace
        )
        .map(job => filterKeys(job, 'TaskGroups', 'NamespaceID'));
    })
  );

  this.post('/jobs', function(schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.Job) return new Response(400, {}, 'Job is a required field on the request payload');

    return okEmpty();
  });

  this.post('/jobs/parse', function(schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.JobHCL)
      return new Response(400, {}, 'JobHCL is a required field on the request payload');
    if (!body.Canonicalize) return new Response(400, {}, 'Expected Canonicalize to be true');

    // Parse the name out of the first real line of HCL to match IDs in the new job record
    // Regex expectation:
    //   in:  job "job-name" {
    //   out: job-name
    const nameFromHCLBlock = /.+?"(.+?)"/;
    const jobName = body.JobHCL.trim()
      .split('\n')[0]
      .match(nameFromHCLBlock)[1];

    const job = server.create('job', { id: jobName });
    return new Response(200, {}, this.serialize(job));
  });

  this.post('/job/:id/plan', function(schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.Job) return new Response(400, {}, 'Job is a required field on the request payload');
    if (!body.Diff) return new Response(400, {}, 'Expected Diff to be true');

    const FailedTGAllocs = body.Job.Unschedulable && generateFailedTGAllocs(body.Job);

    return new Response(
      200,
      {},
      JSON.stringify({ FailedTGAllocs, Diff: generateDiff(req.params.id) })
    );
  });

  this.get(
    '/job/:id',
    withBlockingSupport(function({ jobs }, { params, queryParams }) {
      const job = jobs.all().models.find(job => {
        const jobIsDefault = !job.namespaceId || job.namespaceId === 'default';
        const qpIsDefault = !queryParams.namespace || queryParams.namespace === 'default';
        return (
          job.id === params.id &&
          (job.namespaceId === queryParams.namespace || (jobIsDefault && qpIsDefault))
        );
      });

      return job ? this.serialize(job) : new Response(404, {}, null);
    })
  );

  this.post('/job/:id', function(schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.Job) return new Response(400, {}, 'Job is a required field on the request payload');

    return okEmpty();
  });

  this.get(
    '/job/:id/summary',
    withBlockingSupport(function({ jobSummaries }, { params }) {
      return this.serialize(jobSummaries.findBy({ jobId: params.id }));
    })
  );

  this.get('/job/:id/allocations', function({ allocations }, { params }) {
    return this.serialize(allocations.where({ jobId: params.id }));
  });

  this.get('/job/:id/versions', function({ jobVersions }, { params }) {
    return this.serialize(jobVersions.where({ jobId: params.id }));
  });

  this.get('/job/:id/deployments', function({ deployments }, { params }) {
    return this.serialize(deployments.where({ jobId: params.id }));
  });

  this.get('/job/:id/deployment', function({ deployments }, { params }) {
    const deployment = deployments.where({ jobId: params.id }).models[0];
    return deployment ? this.serialize(deployment) : new Response(200, {}, 'null');
  });

  this.post('/job/:id/periodic/force', function(schema, { params }) {
    // Create the child job
    const parent = schema.jobs.find(params.id);

    // Use the server instead of the schema to leverage the job factory
    server.create('job', 'periodicChild', {
      parentId: parent.id,
      namespaceId: parent.namespaceId,
      namespace: parent.namespace,
      createAllocations: parent.createAllocations,
    });

    return okEmpty();
  });

  this.delete('/job/:id', function(schema, { params }) {
    const job = schema.jobs.find(params.id);
    job.update({ status: 'dead' });
    return new Response(204, {}, '');
  });

  this.get('/deployment/:id');
  this.post('/deployment/promote/:id', function() {
    return new Response(204, {}, '');
  });

  this.get('/job/:id/evaluations', function({ evaluations }, { params }) {
    return this.serialize(evaluations.where({ jobId: params.id }));
  });

  this.get('/evaluation/:id');

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

  this.get('/allocations');

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

  this.get('/agent/members', function({ agents, regions }) {
    const firstRegion = regions.first();
    return {
      ServerRegion: firstRegion ? firstRegion.id : null,
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

  this.get('/regions', function({ regions }) {
    return this.serialize(regions.all());
  });

  const clientAllocationStatsHandler = function({ clientAllocationStats }, { params }) {
    return this.serialize(clientAllocationStats.find(params.id));
  };

  const clientAllocationLog = function(server, { params, queryParams }) {
    const allocation = server.allocations.find(params.allocation_id);
    const tasks = allocation.taskStateIds.map(id => server.taskStates.find(id));

    if (!tasks.mapBy('name').includes(queryParams.task)) {
      return new Response(400, {}, 'must include task name');
    }

    if (queryParams.plain) {
      return logFrames.join('');
    }

    return logEncode(logFrames, logFrames.length - 1);
  };

  // Client requests are available on the server and the client
  this.get('/client/allocation/:id/stats', clientAllocationStatsHandler);
  this.get('/client/fs/logs/:allocation_id', clientAllocationLog);

  this.get('/client/v1/client/stats', function({ clientStats }, { queryParams }) {
    return this.serialize(clientStats.find(queryParams.node_id));
  });

  // TODO: in the future, this hack may be replaceable with dynamic host name
  // support in pretender: https://github.com/pretenderjs/pretender/issues/210
  HOSTS.forEach(host => {
    this.get(`http://${host}/v1/client/allocation/:id/stats`, clientAllocationStatsHandler);
    this.get(`http://${host}/v1/client/fs/logs/:allocation_id`, clientAllocationLog);

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

// An empty response but not a 204 No Content. This is still a valid JSON
// response that represents a payload with no worthwhile data.
function okEmpty() {
  return new Response(200, {}, '{}');
}

function generateFailedTGAllocs(job, taskGroups) {
  const taskGroupsFromSpec = job.TaskGroups && job.TaskGroups.mapBy('Name');

  let tgNames = ['tg-one', 'tg-two'];
  if (taskGroupsFromSpec && taskGroupsFromSpec.length) tgNames = taskGroupsFromSpec;
  if (taskGroups && taskGroups.length) tgNames = taskGroups;

  return tgNames.reduce((hash, tgName) => {
    hash[tgName] = generateTaskGroupFailures();
    return hash;
  }, {});
}
