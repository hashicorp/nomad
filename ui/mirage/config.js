import Ember from 'ember';
import Response from 'ember-cli-mirage/response';
import { HOSTS } from './common';
import { logFrames, logEncode } from './data/logs';
import { generateDiff } from './factories/job-version';
import { generateTaskGroupFailures } from './factories/evaluation';
import { copy } from 'ember-copy';

export function findLeader(schema) {
  const agent = schema.agents.first();
  return `${agent.address}:${agent.tags.port}`;
}

export function filesForPath(allocFiles, filterPath) {
  return allocFiles.where(
    file =>
      (!filterPath || file.path.startsWith(filterPath)) &&
      file.path.length > filterPath.length &&
      !file.path.substr(filterPath.length + 1).includes('/')
  );
}

export default function() {
  this.timing = 0; // delay for each request, automatically set to 0 during testing

  this.logging = window.location.search.includes('mirage-logging=true');

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
        response.headers['x-nomad-index'] = index;
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
        .filter(job =>
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

  this.get(
    '/job/:id/scale',
    withBlockingSupport(function({ jobScales }, { params }) {
      const obj = jobScales.findBy({ jobId: params.id });
      return this.serialize(jobScales.findBy({ jobId: params.id }));
    })
  );

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

  this.post('/job/:id/scale', function({ jobs }, { params }) {
    return this.serialize(jobs.find(params.id));
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

  this.post('/node/:id/eligibility', function({ nodes }, { params, requestBody }) {
    const body = JSON.parse(requestBody);
    const node = nodes.find(params.id);

    node.update({ schedulingEligibility: body.Elibility === 'eligible' });
    return this.serialize(node);
  });

  this.post('/node/:id/drain', function({ nodes }, { params }) {
    return this.serialize(nodes.find(params.id));
  });

  this.get('/allocations');

  this.get('/allocation/:id');

  this.post('/allocation/:id/stop', function() {
    return new Response(204, {}, '');
  });

  this.get(
    '/volumes',
    withBlockingSupport(function({ csiVolumes }, { queryParams }) {
      if (queryParams.type !== 'csi') {
        return new Response(200, {}, '[]');
      }

      const json = this.serialize(csiVolumes.all());
      const namespace = queryParams.namespace || 'default';
      return json.filter(volume =>
        namespace === 'default'
          ? !volume.NamespaceID || volume.NamespaceID === namespace
          : volume.NamespaceID === namespace
      );
    })
  );

  this.get(
    '/volume/:id',
    withBlockingSupport(function({ csiVolumes }, { params }) {
      if (!params.id.startsWith('csi/')) {
        return new Response(404, {}, null);
      }

      const id = params.id.replace(/^csi\//, '');
      const volume = csiVolumes.find(id);

      if (!volume) {
        return new Response(404, {}, null);
      }

      return this.serialize(volume);
    })
  );

  this.get('/plugins', function({ csiPlugins }, { queryParams }) {
    if (queryParams.type !== 'csi') {
      return new Response(200, {}, '[]');
    }

    return this.serialize(csiPlugins.all());
  });

  this.get('/plugin/:id', function({ csiPlugins }, { params }) {
    if (!params.id.startsWith('csi/')) {
      return new Response(404, {}, null);
    }

    const id = params.id.replace(/^csi\//, '');
    const volume = csiPlugins.find(id);

    if (!volume) {
      return new Response(404, {}, null);
    }

    return this.serialize(volume);
  });

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

  this.get('/agent/monitor', function({ agents, nodes }, { queryParams }) {
    const serverId = queryParams.server_id;
    const clientId = queryParams.client_id;

    if (serverId && clientId)
      return new Response(400, {}, 'specify a client or a server, not both');
    if (serverId && !agents.findBy({ name: serverId }))
      return new Response(400, {}, 'specified server does not exist');
    if (clientId && !nodes.find(clientId))
      return new Response(400, {}, 'specified client does not exist');

    if (queryParams.plain) {
      return logFrames.join('');
    }

    return logEncode(logFrames, logFrames.length - 1);
  });

  this.get('/status/leader', function(schema) {
    return JSON.stringify(findLeader(schema));
  });

  this.get('/acl/token/self', function({ tokens }, req) {
    const secret = req.requestHeaders['x-nomad-token'];
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
    const secret = req.requestHeaders['x-nomad-token'];
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
    const secret = req.requestHeaders['x-nomad-token'];
    const tokenForSecret = tokens.findBy({ secretId: secret });

    if (req.params.id === 'anonymous') {
      if (policy) {
        return this.serialize(policy);
      } else {
        return new Response(404, {}, null);
      }
    }

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

  const clientAllocationFSLsHandler = function({ allocFiles }, { queryParams: { path } }) {
    const filterPath = path.endsWith('/') ? path.substr(0, path.length - 1) : path;
    const files = filesForPath(allocFiles, filterPath);
    return this.serialize(files);
  };

  const clientAllocationFSStatHandler = function({ allocFiles }, { queryParams: { path } }) {
    const filterPath = path.endsWith('/') ? path.substr(0, path.length - 1) : path;

    // Root path
    if (!filterPath) {
      return this.serialize({
        IsDir: true,
        ModTime: new Date(),
      });
    }

    // Either a file or a nested directory
    const file = allocFiles.where({ path: filterPath }).models[0];
    return this.serialize(file);
  };

  const clientAllocationCatHandler = function({ allocFiles }, { queryParams }) {
    const [file, err] = fileOrError(allocFiles, queryParams.path);

    if (err) return err;
    return file.body;
  };

  const clientAllocationStreamHandler = function({ allocFiles }, { queryParams }) {
    const [file, err] = fileOrError(allocFiles, queryParams.path);

    if (err) return err;

    // Pretender, and therefore Mirage, doesn't support streaming responses.
    return file.body;
  };

  const clientAllocationReadAtHandler = function({ allocFiles }, { queryParams }) {
    const [file, err] = fileOrError(allocFiles, queryParams.path);

    if (err) return err;
    return file.body.substr(queryParams.offset || 0, queryParams.limit);
  };

  const fileOrError = function(allocFiles, path, message = 'Operation not allowed on a directory') {
    // Root path
    if (path === '/') {
      return [null, new Response(400, {}, message)];
    }

    const file = allocFiles.where({ path }).models[0];
    if (file.isDir) {
      return [null, new Response(400, {}, message)];
    }

    return [file, null];
  };

  // Client requests are available on the server and the client
  this.put('/client/allocation/:id/restart', function() {
    return new Response(204, {}, '');
  });

  this.get('/client/allocation/:id/stats', clientAllocationStatsHandler);
  this.get('/client/fs/logs/:allocation_id', clientAllocationLog);

  this.get('/client/fs/ls/:allocation_id', clientAllocationFSLsHandler);
  this.get('/client/fs/stat/:allocation_id', clientAllocationFSStatHandler);
  this.get('/client/fs/cat/:allocation_id', clientAllocationCatHandler);
  this.get('/client/fs/stream/:allocation_id', clientAllocationStreamHandler);
  this.get('/client/fs/readat/:allocation_id', clientAllocationReadAtHandler);

  this.get('/client/stats', function({ clientStats }, { queryParams }) {
    const seed = faker.random.number(10);
    if (seed >= 8) {
      const stats = clientStats.find(queryParams.node_id);
      stats.update({
        timestamp: Date.now() * 1000000,
        CPUTicksConsumed: stats.CPUTicksConsumed + faker.random.number({ min: -10, max: 10 }),
      });
      return this.serialize(stats);
    } else {
      return new Response(500, {}, null);
    }
  });

  // TODO: in the future, this hack may be replaceable with dynamic host name
  // support in pretender: https://github.com/pretenderjs/pretender/issues/210
  HOSTS.forEach(host => {
    this.get(`http://${host}/v1/client/allocation/:id/stats`, clientAllocationStatsHandler);
    this.get(`http://${host}/v1/client/fs/logs/:allocation_id`, clientAllocationLog);

    this.get(`http://${host}/v1/client/fs/ls/:allocation_id`, clientAllocationFSLsHandler);
    this.get(`http://${host}/v1/client/stat/ls/:allocation_id`, clientAllocationFSStatHandler);
    this.get(`http://${host}/v1/client/fs/cat/:allocation_id`, clientAllocationCatHandler);
    this.get(`http://${host}/v1/client/fs/stream/:allocation_id`, clientAllocationStreamHandler);
    this.get(`http://${host}/v1/client/fs/readat/:allocation_id`, clientAllocationReadAtHandler);

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
