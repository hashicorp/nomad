/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import Response from 'ember-cli-mirage/response';
import { HOSTS } from './common';
import { logFrames, logEncode } from './data/logs';
import { generateDiff } from './factories/job-version';
import { generateTaskGroupFailures } from './factories/evaluation';
import { copy } from 'ember-copy';
import formatHost from 'nomad-ui/utils/format-host';
import faker from 'nomad-ui/mirage/faker';

export function findLeader(schema) {
  const agent = schema.agents.first();
  return formatHost(agent.member.Address, agent.member.Tags.port);
}

export function filesForPath(allocFiles, filterPath) {
  return allocFiles.where(
    (file) =>
      (!filterPath || file.path.startsWith(filterPath)) &&
      file.path.length > filterPath.length &&
      !file.path.substr(filterPath.length + 1).includes('/')
  );
}

export default function () {
  this.timing = 0; // delay for each request, automatically set to 0 during testing

  this.logging = window.location.search.includes('mirage-logging=true');

  this.namespace = 'v1';
  this.trackRequests = Ember.testing;

  const nomadIndices = {}; // used for tracking blocking queries
  const server = this;
  const withBlockingSupport = function (
    fn,
    { pagination = false, tokenProperty = 'ModifyIndex' } = {}
  ) {
    return function (schema, request) {
      let handler = fn;
      if (pagination) {
        handler = withPagination(handler, tokenProperty);
      }

      // Get the original response
      let { url } = request;
      url = url.replace(/index=\d+[&;]?/, '');
      let response = handler.apply(this, arguments);

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

  const withPagination = function (fn, tokenProperty = 'ModifyIndex') {
    return function (schema, request) {
      let response = fn.apply(this, arguments);
      if (response.code && response.code !== 200) {
        return response;
      }
      let perPage = parseInt(request.queryParams.per_page || 25);
      let page = parseInt(request.queryParams.page || 1);
      let totalItems = response.length;
      let totalPages = Math.ceil(totalItems / perPage);
      let hasMore = page < totalPages;

      let paginatedItems = response.slice((page - 1) * perPage, page * perPage);

      let nextToken = null;
      if (hasMore) {
        nextToken = response[page * perPage][tokenProperty];
      }

      if (nextToken) {
        return new Response(
          200,
          { 'x-nomad-nexttoken': nextToken },
          paginatedItems
        );
      } else {
        return new Response(200, {}, paginatedItems);
      }
    };
  };

  this.get(
    '/jobs',
    withBlockingSupport(function ({ jobs }, { queryParams }) {
      const json = this.serialize(jobs.all());
      const namespace = queryParams.namespace || 'default';
      return json
        .filter((job) => {
          if (namespace === '*') return true;
          return namespace === 'default'
            ? !job.NamespaceID || job.NamespaceID === namespace
            : job.NamespaceID === namespace;
        })
        .map((job) => filterKeys(job, 'TaskGroups', 'NamespaceID'));
    })
  );

  this.get(
    '/jobs/statuses',
    withBlockingSupport(
      function ({ jobs }, req) {
        const namespace = req.queryParams.namespace || '*';
        let nextToken = req.queryParams.next_token || 0;
        let reverse = req.queryParams.reverse === 'true';
        const json = this.serialize(jobs.all());

        // Let's implement a very basic handling of ?filter here.
        // We'll assume at most "and" combinations, and only positive filters
        // (no "not Type contains sys" or similar)
        let filteredJson = json;

        // Filter out all child jobs
        if (!req.queryParams.include_children) {
          filteredJson = filteredJson.filter((job) => !job.ParentID);
        }

        if (req.queryParams.filter) {
          // Format will be something like "Name contains NAME" or "Type == sysbatch" or combinations thereof
          const filterConditions = req.queryParams.filter
            .split(' and ')
            .map((condition) => {
              // Dropdowns user parenthesis wrapping; remove them for mock/test purposes
              // We want to test multiple conditions within parens, like "(Type == system or Type == sysbatch)"
              // So if we see parenthesis, we should re-split on "or" and treat them as separate conditions.
              if (condition.startsWith('(') && condition.endsWith(')')) {
                condition = condition.slice(1, -1);
              }
              if (condition.includes(' or ')) {
                // multiple or condition
                return {
                  field: condition.split(' ')[0],
                  operator: '==',
                  parts: condition.split(' or ').map((part) => {
                    return (
                      part
                        .split(' ')[2]
                        // mirage only: strip quotes
                        .replace(/['"]+/g, '')
                    );
                  }),
                };
              } else {
                const parts = condition.split(' ');

                return {
                  field: parts[0],
                  operator: parts[1],
                  value: parts.slice(2).join(' ').replace(/['"]+/g, ''),
                };
              }
            });

          const allowedFields = [
            'Name',
            'Status',
            'StatusDescription',
            'Region',
            'NodePool',
            'Namespace',
            'Version',
            'Priority',
            'Stop',
            'Type',
            'ID',
            'AllAtOnce',
            'Datacenters',
            'Dispatched',
            'ConsulToken',
            'ConsulNamespace',
            'VaultToken',
            'VaultNamespace',
            'NomadTokenID',
            'Stable',
            'SubmitTime',
            'CreateIndex',
            'ModifyIndex',
            'JobModifyIndex',
            'ParentID',
          ];

          // Simulate a failure if a filterCondition's field is not among the allowed
          if (
            !filterConditions.every((condition) =>
              allowedFields.includes(condition.field)
            )
          ) {
            return new Response(
              500,
              {},
              'couldn\'t find key: struct field with name "' +
                filterConditions[0].field +
                '" in type (...extraneous)'
            );
          }

          filteredJson = filteredJson.filter((job) => {
            return filterConditions.every((condition) => {
              if (condition.parts) {
                // Making a shortcut assumption that any condition.parts situations
                // will be == as operator for testing sake.
                return condition.parts.some((part) => {
                  return job[condition.field] === part;
                });
              }
              if (condition.operator === 'contains') {
                return (
                  job[condition.field] &&
                  job[condition.field].includes(condition.value)
                );
              } else if (condition.operator === '==') {
                return job[condition.field] === condition.value;
              } else if (condition.operator === '!=') {
                return job[condition.field] !== condition.value;
              }
              return true;
            });
          });
        }

        let sortedJson = filteredJson
          .sort((a, b) =>
            reverse
              ? a.ModifyIndex - b.ModifyIndex
              : b.ModifyIndex - a.ModifyIndex
          )
          .filter((job) => {
            if (namespace === '*') return true;
            return namespace === 'default'
              ? !job.NamespaceID || job.NamespaceID === 'default'
              : job.NamespaceID === namespace;
          })
          .map((job) => filterKeys(job, 'TaskGroups', 'NamespaceID'));
        if (nextToken) {
          sortedJson = sortedJson.filter((job) =>
            reverse
              ? job.ModifyIndex >= nextToken
              : job.ModifyIndex <= nextToken
          );
        }
        return sortedJson;
      },
      { pagination: true, tokenProperty: 'ModifyIndex' }
    )
  );

  this.post(
    '/jobs/statuses',
    withBlockingSupport(function ({ jobs }, req) {
      const body = JSON.parse(req.requestBody);
      const requestedJobs = body.jobs || [];
      const allJobs = this.serialize(jobs.all());

      let returnedJobs = allJobs
        .filter((job) => {
          return requestedJobs.some((requestedJob) => {
            return (
              job.ID === requestedJob.id &&
              (requestedJob.namespace === 'default' ||
                job.NamespaceID === requestedJob.namespace)
            );
          });
        })
        .map((j) => {
          let job = {};

          // get children that may have been created
          let children = null;
          if (j.Periodic || j.Parameterized) {
            children = allJobs.filter((child) => {
              return child.ParentID === j.ID;
            });
          }
          job.ID = j.ID;
          job.Name = j.Name;
          job.ModifyIndex = j.ModifyIndex;
          job.Allocs = server.db.allocations
            .where({ jobId: j.ID, namespace: j.Namespace })
            .map((alloc) => {
              let taskStates = server.db.taskStates.where({
                allocationId: alloc.id,
              });
              return {
                ClientStatus: alloc.clientStatus,
                DeploymentStatus: {
                  Canary: false,
                  Healthy: true,
                },
                Group: alloc.taskGroup,
                JobVersion: alloc.jobVersion,
                NodeID: alloc.nodeId,
                ID: alloc.id,
                HasPausedTask: taskStates.any((ts) => ts.paused),
              };
            });
          job.ChildStatuses = children ? children.mapBy('Status') : null;
          job.Datacenters = j.Datacenters;
          job.LatestDeployment = j.LatestDeployment;
          job.GroupCountSum = j.TaskGroups.mapBy('Count').reduce(
            (a, b) => a + b,
            0
          );
          job.Namespace = j.NamespaceID;
          job.NodePool = j.NodePool;
          job.Type = j.Type;
          job.Priority = j.Priority;
          job.Version = j.Version;
          return job;
        });
      // sort by modifyIndex, descending
      returnedJobs
        .sort((a, b) => b.ID.localeCompare(a.ID))
        .sort((a, b) => b.ModifyIndex - a.ModifyIndex);

      return returnedJobs;
    })
  );

  this.post('/jobs', function (schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.Job)
      return new Response(
        400,
        {},
        'Job is a required field on the request payload'
      );

    return okEmpty();
  });

  this.post('/jobs/parse', function (schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.JobHCL)
      return new Response(
        400,
        {},
        'JobHCL is a required field on the request payload'
      );
    if (!body.Canonicalize)
      return new Response(400, {}, 'Expected Canonicalize to be true');

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

  this.get('/job/:id/submission', function (schema, req) {
    return new Response(
      200,
      {},
      JSON.stringify({
        Source: `job "${req.params.id}" {`,
        Format: 'hcl2',
        VariableFlags: { X: 'x', Y: '42', Z: 'true' },
        Variables: 'var file content',
      })
    );
  });

  this.post('/job/:id/plan', function (schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.Job)
      return new Response(
        400,
        {},
        'Job is a required field on the request payload'
      );
    if (!body.Diff) return new Response(400, {}, 'Expected Diff to be true');

    const FailedTGAllocs =
      body.Job.Unschedulable && generateFailedTGAllocs(body.Job);

    const jobPlanWarnings = body.Job.WithWarnings && generateWarnings();

    return new Response(
      200,
      {},
      JSON.stringify({
        FailedTGAllocs,
        Warnings: jobPlanWarnings,
        Diff: generateDiff(req.params.id),
      })
    );
  });

  this.get(
    '/job/:id',
    withBlockingSupport(function ({ jobs }, { params, queryParams }) {
      const job = jobs.all().models.find((job) => {
        const jobIsDefault = !job.namespaceId || job.namespaceId === 'default';
        const qpIsDefault =
          !queryParams.namespace || queryParams.namespace === 'default';
        return (
          job.id === params.id &&
          (job.namespaceId === queryParams.namespace ||
            (jobIsDefault && qpIsDefault))
        );
      });

      return job ? this.serialize(job) : new Response(404, {}, null);
    })
  );

  this.post('/job/:id', function (schema, req) {
    const body = JSON.parse(req.requestBody);

    if (!body.Job)
      return new Response(
        400,
        {},
        'Job is a required field on the request payload'
      );

    return okEmpty();
  });

  this.get(
    '/job/:id/summary',
    withBlockingSupport(function ({ jobSummaries }, { params }) {
      return this.serialize(jobSummaries.findBy({ jobId: params.id }));
    })
  );

  this.get('/job/:id/allocations', function ({ allocations }, { params }) {
    return this.serialize(allocations.where({ jobId: params.id }));
  });

  this.get('/job/:id/versions', function ({ jobVersions }, { params }) {
    return this.serialize(jobVersions.where({ jobId: params.id }));
  });

  this.post(
    '/job/:id/versions/:version/tag',
    function ({ jobVersions }, { params }) {
      // Create a new version tag
      const tag = server.create('version-tag', {
        jobVersion: jobVersions.findBy({
          jobId: params.id,
          version: params.version,
        }),
        name: params.name,
        description: params.description,
      });
      return this.serialize(tag);
    }
  );

  this.delete(
    '/job/:id/versions/:version/tag',
    function ({ jobVersions }, { params }) {
      return this.serialize(
        jobVersions.findBy({ jobId: params.id, version: params.version })
      );
    }
  );

  this.get('/job/:id/deployments', function ({ deployments }, { params }) {
    return this.serialize(deployments.where({ jobId: params.id }));
  });

  this.get('/job/:id/deployment', function ({ deployments }, { params }) {
    const deployment = deployments.where({ jobId: params.id }).models[0];
    return deployment
      ? this.serialize(deployment)
      : new Response(200, {}, 'null');
  });

  this.get(
    '/job/:id/scale',
    withBlockingSupport(function ({ jobScales }, { params }) {
      const obj = jobScales.findBy({ jobId: params.id });
      return this.serialize(jobScales.findBy({ jobId: params.id }));
    })
  );

  this.post('/job/:id/periodic/force', function (schema, { params }) {
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

  this.post('/job/:id/dispatch', function (schema, { params }) {
    // Create the child job
    const parent = schema.jobs.find(params.id);

    // Use the server instead of the schema to leverage the job factory
    let dispatched = server.create('job', 'parameterizedChild', {
      parentId: parent.id,
      namespaceId: parent.namespaceId,
      namespace: parent.namespace,
      createAllocations: parent.createAllocations,
    });

    return new Response(
      200,
      {},
      JSON.stringify({
        DispatchedJobID: dispatched.id,
      })
    );
  });

  this.post('/job/:id/revert', function ({ jobs }, { requestBody }) {
    const { JobID, JobVersion } = JSON.parse(requestBody);
    const job = jobs.find(JobID);
    job.version = JobVersion;
    job.save();

    return okEmpty();
  });

  this.post('/job/:id/scale', function ({ jobs }, { params }) {
    return this.serialize(jobs.find(params.id));
  });

  this.delete('/job/:id', function (schema, { params }) {
    const job = schema.jobs.find(params.id);
    job.update({ status: 'dead' });
    return new Response(204, {}, '');
  });

  this.get('/deployment/:id');

  this.post('/deployment/fail/:id', function () {
    return new Response(204, {}, '');
  });

  this.post('/deployment/promote/:id', function () {
    return new Response(204, {}, '');
  });

  this.get('/job/:id/evaluations', function ({ evaluations }, { params }) {
    return this.serialize(evaluations.where({ jobId: params.id }));
  });

  this.get('/evaluations');
  this.get('/evaluation/:id', function ({ evaluations }, { params }) {
    return evaluations.find(params.id);
  });

  this.get('/deployment/allocations/:id', function (schema, { params }) {
    const job = schema.jobs.find(schema.deployments.find(params.id).jobId);
    const allocations = schema.allocations.where({ jobId: job.id });

    return this.serialize(allocations.slice(0, 3));
  });

  this.get('/nodes', function ({ nodes }, req) {
    // authorize user permissions
    const token = server.db.tokens.findBy({
      secretId: req.requestHeaders['X-Nomad-Token'],
    });

    if (token) {
      const policyIds = token.policyIds || [];

      const roleIds = token.roleIds || [];
      const roles = server.db.roles.find(roleIds);
      const rolePolicyIds = roles.map((role) => role.policyIds).flat();

      const policies = server.db.policies.find([
        ...policyIds,
        ...rolePolicyIds,
      ]);
      const hasReadPolicy = policies.find(
        (p) =>
          p.rulesJSON.Node?.Policy === 'read' ||
          p.rulesJSON.Node?.Policy === 'write'
      );
      if (hasReadPolicy) {
        const json = this.serialize(nodes.all());
        return json;
      }
      return new Response(403, {}, 'Permissions have not be set-up.');
    }

    // TODO:  Think about policy handling in Mirage set-up
    return this.serialize(nodes.all());
  });

  this.get('/node/:id');

  this.get('/node/:id/allocations', function ({ allocations }, { params }) {
    return this.serialize(allocations.where({ nodeId: params.id }));
  });

  this.post(
    '/node/:id/eligibility',
    function ({ nodes }, { params, requestBody }) {
      const body = JSON.parse(requestBody);
      const node = nodes.find(params.id);

      node.update({ schedulingEligibility: body.Elibility === 'eligible' });
      return this.serialize(node);
    }
  );

  this.post('/node/:id/drain', function ({ nodes }, { params }) {
    return this.serialize(nodes.find(params.id));
  });

  this.get('/node/pools', function ({ nodePools }) {
    return this.serialize(nodePools.all());
  });

  this.get('/allocations');

  this.get('/allocation/:id');

  this.post('/allocation/:id/stop', function () {
    return new Response(204, {}, '');
  });

  this.get(
    '/volumes',
    withBlockingSupport(function ({ csiVolumes }, { queryParams }) {
      if (queryParams.type !== 'csi') {
        return new Response(200, {}, '[]');
      }

      const json = this.serialize(csiVolumes.all());
      const namespace = queryParams.namespace || 'default';
      return json.filter((volume) => {
        if (namespace === '*') return true;
        return namespace === 'default'
          ? !volume.NamespaceID || volume.NamespaceID === namespace
          : volume.NamespaceID === namespace;
      });
    })
  );

  this.get(
    '/volume/:id',
    withBlockingSupport(function ({ csiVolumes }, { params, queryParams }) {
      if (!params.id.startsWith('csi/')) {
        return new Response(404, {}, null);
      }

      const id = params.id.replace(/^csi\//, '');
      const volume = csiVolumes.all().models.find((volume) => {
        const volumeIsDefault =
          !volume.namespaceId || volume.namespaceId === 'default';
        const qpIsDefault =
          !queryParams.namespace || queryParams.namespace === 'default';
        return (
          volume.id === id &&
          (volume.namespaceId === queryParams.namespace ||
            (volumeIsDefault && qpIsDefault))
        );
      });

      return volume ? this.serialize(volume) : new Response(404, {}, null);
    })
  );

  this.get('/plugins', function ({ csiPlugins }, { queryParams }) {
    if (queryParams.type !== 'csi') {
      return new Response(200, {}, '[]');
    }

    return this.serialize(csiPlugins.all());
  });

  this.get('/plugin/csi/:id', function ({ csiPlugins }, { params }) {
    const volume = csiPlugins.find(params.id);

    if (!volume) {
      return new Response(404, {}, null);
    }

    return this.serialize(volume);
  });

  this.get('/agent/members', function ({ agents, regions }) {
    const firstRegion = regions.first();
    return {
      ServerRegion: firstRegion ? firstRegion.id : null,
      Members: this.serialize(agents.all()).map(({ member }) => ({
        ...member,
      })),
    };
  });

  this.get('/agent/self', function ({ agents }) {
    return agents.first();
  });

  this.get('/agent/monitor', function ({ agents, nodes }, { queryParams }) {
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

  this.get('/status/leader', function (schema) {
    return JSON.stringify(findLeader(schema));
  });

  this.get('/acl/tokens', function ({ tokens }, req) {
    return this.serialize(tokens.all());
  });

  this.delete('/acl/token/:id', function (schema, request) {
    const { id } = request.params;
    server.db.tokens.remove(id);
    return '';
  });

  this.post('/acl/token', function (schema, request) {
    const { Name, Policies, Type, ExpirationTTL, ExpirationTime, Global } =
      JSON.parse(request.requestBody);

    function parseDuration(duration) {
      const [_, value, unit] = duration.match(/(\d+)(\w)/);
      const unitMap = {
        s: 1000,
        m: 1000 * 60,
        h: 1000 * 60 * 60,
        d: 1000 * 60 * 60 * 24,
      };
      return value * unitMap[unit];
    }

    // If there's an expirationTime, use that. Otherwise, use the TTL.
    const expirationTime = ExpirationTime
      ? new Date(ExpirationTime)
      : ExpirationTTL
      ? new Date(Date.now() + parseDuration(ExpirationTTL))
      : null;

    return server.create('token', {
      name: Name,
      policyIds: Policies,
      type: Type,
      id: faker.random.uuid(),
      expirationTime,
      global: Global,
      createTime: new Date().toISOString(),
    });
  });

  this.post('/acl/token/:id', function (schema, request) {
    // If both Policies and Roles arrays are empty, return an error
    const { Policies, Roles } = JSON.parse(request.requestBody);
    if (!Policies.length && !Roles.length) {
      return new Response(
        500,
        {},
        'Either Policies or Roles must be specified'
      );
    }
    return new Response(
      200,
      {},
      {
        id: request.params.id,
        Policies,
        Roles,
      }
    );
  });

  this.get('/acl/token/self', function ({ tokens }, req) {
    const secret = req.requestHeaders['X-Nomad-Token'];
    const tokenForSecret = tokens.findBy({ secretId: secret });

    // Return the token if it exists
    if (tokenForSecret) {
      return this.serialize(tokenForSecret);
    }

    // Client error if it doesn't
    return new Response(400, {}, null);
  });

  this.post('/acl/login', function (schema, { requestBody }) {
    const { LoginToken } = JSON.parse(requestBody);
    const tokenType = LoginToken.endsWith('management')
      ? 'management'
      : 'client';
    const isBad = LoginToken.endsWith('bad');

    if (isBad) {
      return new Response(403, {}, null);
    } else {
      const token = schema.tokens
        .all()
        .models.find((token) => token.type === tokenType);
      return this.serialize(token);
    }
  });

  this.get('/acl/token/:id', function ({ tokens }, req) {
    const token = tokens.find(req.params.id);
    const secret = req.requestHeaders['X-Nomad-Token'];
    const tokenForSecret = tokens.findBy({ secretId: secret });

    // Return the token only if the request header matches the token
    // or the token is of type management
    if (
      token.secretId === secret ||
      (tokenForSecret && tokenForSecret.type === 'management')
    ) {
      return this.serialize(token);
    }

    // Return not authorized otherwise
    return new Response(403, {}, null);
  });

  this.post(
    '/acl/token/onetime/exchange',
    function ({ tokens }, { requestBody }) {
      const { OneTimeSecretID } = JSON.parse(requestBody);

      const tokenForSecret = tokens.findBy({ oneTimeSecret: OneTimeSecretID });

      // Return the token if it exists
      if (tokenForSecret) {
        return {
          Token: this.serialize(tokenForSecret),
        };
      }

      // Forbidden error if it doesn't
      return new Response(403, {}, null);
    }
  );

  this.get('/acl/policy/:id', function ({ policies, tokens }, req) {
    const policy = policies.findBy({ name: req.params.id });
    const secret = req.requestHeaders['X-Nomad-Token'];
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
      (tokenForSecret.policies.includes(policy) ||
        tokenForSecret.roles.models.any((role) =>
          role.policies.includes(policy)
        ) ||
        tokenForSecret.type === 'management')
    ) {
      return this.serialize(policy);
    }

    // Return not authorized otherwise
    return new Response(403, {}, null);
  });

  this.get('/acl/roles', function ({ roles }, req) {
    return this.serialize(roles.all());
  });

  this.get('/acl/role/:id', function ({ roles }, req) {
    const role = roles.findBy({ id: req.params.id });
    return this.serialize(role);
  });

  this.post('/acl/role', function (schema, request) {
    const { Name, Description } = JSON.parse(request.requestBody);
    return server.create('role', {
      name: Name,
      description: Description,
    });
  });

  this.put('/acl/role/:id', function (schema, request) {
    const { Policies } = JSON.parse(request.requestBody);
    if (!Policies.length) {
      return new Response(500, {}, 'Policies must be specified');
    }
    return new Response(
      200,
      {},
      {
        id: request.params.id,
        Policies,
      }
    );
  });

  this.delete('/acl/role/:id', function (schema, request) {
    const { id } = request.params;

    // Also update any tokens whose policyIDs include this policy
    const tokens =
      server.schema.tokens.where((token) => token.roleIds?.includes(id)) || [];
    tokens.models.forEach((token) => {
      token.update({
        roleIds: token.roleIds.filter((roleId) => roleId !== id),
      });
    });

    server.db.roles.remove(id);
    return '';
  });

  this.get('/acl/policies', function ({ policies }, req) {
    return this.serialize(policies.all());
  });

  this.get('/sentinel/policies', function (schema, req) {
    return this.serialize(schema.sentinelPolicies.all());
  });

  this.post('/sentinel/policy/:id', function (schema, req) {
    const { Name, Description, EnforcementLevel, Policy, Scope } = JSON.parse(
      req.requestBody
    );
    return server.create('sentinelPolicy', {
      name: Name,
      description: Description,
      enforcementLevel: EnforcementLevel,
      policy: Policy,
      scope: Scope,
    });
  });

  this.get('/sentinel/policy/:id', function ({ sentinelPolicies }, req) {
    return this.serialize(sentinelPolicies.findBy({ name: req.params.id }));
  });

  this.delete('/sentinel/policy/:id', function (schema, req) {
    const { id } = req.params;
    server.db.sentinelPolicies.remove(id);
    return '';
  });

  this.put('/sentinel/policy/:id', function (schema, req) {
    return new Response(200, {}, {});
  });

  this.delete('/acl/policy/:id', function (schema, request) {
    const { id } = request.params;

    // Also update any tokens whose policyIDs include this policy
    const tokens =
      server.schema.tokens.where((token) => token.policyIds?.includes(id)) ||
      [];
    tokens.models.forEach((token) => {
      token.update({
        policyIds: token.policyIds.filter((policyId) => policyId !== id),
      });
    });

    // Also update any roles whose policyIDs include this policy
    const roles =
      server.schema.roles.where((role) => role.policyIds?.includes(id)) || [];
    roles.models.forEach((role) => {
      role.update({
        policyIds: role.policyIds.filter((policyId) => policyId !== id),
      });
    });

    server.db.policies.remove(id);

    return '';
  });

  this.put('/acl/policy/:id', function (schema, request) {
    return new Response(200, {}, {});
  });

  this.post('/acl/policy/:id', function (schema, request) {
    const { Name, Description, Rules } = JSON.parse(request.requestBody);
    return server.create('policy', {
      name: Name,
      description: Description,
      rules: Rules,
    });
  });

  this.get('/namespaces', function ({ namespaces }) {
    const records = namespaces.all();

    if (records.length) {
      return this.serialize(records);
    }

    return this.serialize([{ Name: 'default' }]);
  });

  this.get('/namespace/:id', function ({ namespaces }, { params }) {
    return this.serialize(namespaces.find(params.id));
  });

  this.post('/namespace/:id', function (schema, request) {
    const { Name, Description } = JSON.parse(request.requestBody);

    return server.create('namespace', {
      id: Name,
      name: Name,
      description: Description,
    });
  });

  this.put('/namespace/:id', function () {
    return new Response(200, {}, {});
  });

  this.delete('/namespace/:id', function (schema, request) {
    const { id } = request.params;

    // If any variables exist for the namespace, error
    const variables =
      server.db.variables.where((v) => v.namespace === id) || [];
    if (variables.length) {
      return new Response(403, {}, 'Namespace has variables');
    }

    server.db.namespaces.remove(id);
    return '';
  });

  this.get('/regions', function ({ regions }) {
    return this.serialize(regions.all());
  });

  this.get('/operator/license', function ({ features }) {
    const records = features.all();

    if (records.length) {
      return {
        License: {
          Features: records.models.mapBy('name'),
        },
      };
    }

    return new Response(501, {}, null);
  });

  const clientAllocationStatsHandler = function (
    { clientAllocationStats },
    { params }
  ) {
    return this.serialize(clientAllocationStats.find(params.id));
  };

  const clientAllocationLog = function (server, { params, queryParams }) {
    const allocation = server.allocations.find(params.allocation_id);
    const tasks = allocation.taskStateIds.map((id) =>
      server.taskStates.find(id)
    );

    if (!tasks.mapBy('name').includes(queryParams.task)) {
      return new Response(400, {}, 'must include task name');
    }

    if (queryParams.plain) {
      return logFrames.join('');
    }

    return logEncode(logFrames, logFrames.length - 1);
  };

  const clientAllocationFSLsHandler = function (
    { allocFiles },
    { queryParams: { path } }
  ) {
    const filterPath = path.endsWith('/')
      ? path.substr(0, path.length - 1)
      : path;
    const files = filesForPath(allocFiles, filterPath);
    return this.serialize(files);
  };

  const clientAllocationFSStatHandler = function (
    { allocFiles },
    { queryParams: { path } }
  ) {
    const filterPath = path.endsWith('/')
      ? path.substr(0, path.length - 1)
      : path;

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

  const clientAllocationCatHandler = function (
    { allocFiles },
    { queryParams }
  ) {
    const [file, err] = fileOrError(allocFiles, queryParams.path);

    if (err) return err;
    return file.body;
  };

  const clientAllocationStreamHandler = function (
    { allocFiles },
    { queryParams }
  ) {
    const [file, err] = fileOrError(allocFiles, queryParams.path);

    if (err) return err;

    // Pretender, and therefore Mirage, doesn't support streaming responses.
    return file.body;
  };

  const clientAllocationReadAtHandler = function (
    { allocFiles },
    { queryParams }
  ) {
    const [file, err] = fileOrError(allocFiles, queryParams.path);

    if (err) return err;
    return file.body.substr(queryParams.offset || 0, queryParams.limit);
  };

  const fileOrError = function (
    allocFiles,
    path,
    message = 'Operation not allowed on a directory'
  ) {
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
  this.put('/client/allocation/:id/restart', function () {
    return new Response(204, {}, '');
  });

  this.get('/client/allocation/:id/stats', clientAllocationStatsHandler);
  this.get('/client/fs/logs/:allocation_id', clientAllocationLog);

  this.get('/client/fs/ls/:allocation_id', clientAllocationFSLsHandler);
  this.get('/client/fs/stat/:allocation_id', clientAllocationFSStatHandler);
  this.get('/client/fs/cat/:allocation_id', clientAllocationCatHandler);
  this.get('/client/fs/stream/:allocation_id', clientAllocationStreamHandler);
  this.get('/client/fs/readat/:allocation_id', clientAllocationReadAtHandler);

  this.get('/client/stats', function ({ clientStats }, { queryParams }) {
    const seed = faker.random.number(10);
    if (seed >= 8) {
      const stats = clientStats.find(queryParams.node_id);
      stats.update({
        timestamp: Date.now() * 1000000,
        CPUTicksConsumed:
          stats.CPUTicksConsumed + faker.random.number({ min: -10, max: 10 }),
      });
      return this.serialize(stats);
    } else {
      return new Response(500, {}, null);
    }
  });

  // Metadata
  this.post(
    '/client/metadata',
    function (schema, { queryParams: { node_id }, requestBody }) {
      const attrs = JSON.parse(requestBody);
      const node = schema.nodes.find(node_id);
      Object.entries(attrs.Meta).forEach(([key, value]) => {
        if (value === null) {
          delete node.meta[key];
          delete attrs.Meta[key];
        }
      });
      return { Meta: { ...node.meta, ...attrs.Meta } };
    }
  );

  // TODO: in the future, this hack may be replaceable with dynamic host name
  // support in pretender: https://github.com/pretenderjs/pretender/issues/210
  HOSTS.forEach((host) => {
    this.get(
      `http://${host}/v1/client/allocation/:id/stats`,
      clientAllocationStatsHandler
    );
    this.get(
      `http://${host}/v1/client/fs/logs/:allocation_id`,
      clientAllocationLog
    );

    this.get(
      `http://${host}/v1/client/fs/ls/:allocation_id`,
      clientAllocationFSLsHandler
    );
    this.get(
      `http://${host}/v1/client/stat/ls/:allocation_id`,
      clientAllocationFSStatHandler
    );
    this.get(
      `http://${host}/v1/client/fs/cat/:allocation_id`,
      clientAllocationCatHandler
    );
    this.get(
      `http://${host}/v1/client/fs/stream/:allocation_id`,
      clientAllocationStreamHandler
    );
    this.get(
      `http://${host}/v1/client/fs/readat/:allocation_id`,
      clientAllocationReadAtHandler
    );

    this.get(`http://${host}/v1/client/stats`, function ({ clientStats }) {
      return this.serialize(clientStats.find(host));
    });
  });

  this.post(
    '/search/fuzzy',
    function (
      { allocations, jobs, nodes, taskGroups, csiPlugins },
      { requestBody }
    ) {
      const { Text } = JSON.parse(requestBody);

      const matchedAllocs = allocations.where((allocation) =>
        allocation.name.includes(Text)
      );
      const matchedGroups = taskGroups.where((taskGroup) =>
        taskGroup.name.includes(Text)
      );
      const matchedJobs = jobs.where((job) => job.name.includes(Text));
      const matchedNodes = nodes.where((node) => node.name.includes(Text));
      const matchedPlugins = csiPlugins.where((plugin) =>
        plugin.id.includes(Text)
      );

      const transformedAllocs = matchedAllocs.models.map((alloc) => ({
        ID: alloc.name,
        Scope: [alloc.namespace || 'default', alloc.id],
      }));

      const transformedGroups = matchedGroups.models.map((group) => ({
        ID: group.name,
        Scope: [group.job.namespace, group.job.id],
      }));

      const transformedJobs = matchedJobs.models.map((job) => ({
        ID: job.name,
        Scope: [job.namespace || 'default', job.id],
      }));

      const transformedNodes = matchedNodes.models.map((node) => ({
        ID: node.name,
        Scope: [node.id],
      }));

      const transformedPlugins = matchedPlugins.models.map((plugin) => ({
        ID: plugin.id,
      }));

      const truncatedAllocs = transformedAllocs.slice(0, 20);
      const truncatedGroups = transformedGroups.slice(0, 20);
      const truncatedJobs = transformedJobs.slice(0, 20);
      const truncatedNodes = transformedNodes.slice(0, 20);
      const truncatedPlugins = transformedPlugins.slice(0, 20);

      return {
        Matches: {
          allocs: truncatedAllocs,
          groups: truncatedGroups,
          jobs: truncatedJobs,
          nodes: truncatedNodes,
          plugins: truncatedPlugins,
        },
        Truncations: {
          allocs: truncatedAllocs.length < truncatedAllocs.length,
          groups: truncatedGroups.length < transformedGroups.length,
          jobs: truncatedJobs.length < transformedJobs.length,
          nodes: truncatedNodes.length < transformedNodes.length,
          plugins: truncatedPlugins.length < transformedPlugins.length,
        },
      };
    }
  );

  this.get(
    '/recommendations',
    function (
      { jobs, namespaces, recommendations },
      { queryParams: { job: id, namespace } }
    ) {
      if (id) {
        if (!namespaces.all().length) {
          namespace = null;
        }

        const job = jobs.findBy({ id, namespace });

        if (!job) {
          return [];
        }

        const taskGroups = job.taskGroups.models;

        const tasks = taskGroups.reduce((tasks, taskGroup) => {
          return tasks.concat(taskGroup.tasks.models);
        }, []);

        const recommendationIds = tasks.reduce((recommendationIds, task) => {
          return recommendationIds.concat(
            task.recommendations.models.mapBy('id')
          );
        }, []);

        return recommendations.find(recommendationIds);
      } else {
        return recommendations.all();
      }
    }
  );

  this.post(
    '/recommendations/apply',
    function ({ recommendations }, { requestBody }) {
      const { Apply, Dismiss } = JSON.parse(requestBody);

      Apply.concat(Dismiss).forEach((id) => {
        const recommendation = recommendations.find(id);
        const task = recommendation.task;

        if (Apply.includes(id)) {
          task.resources[recommendation.resource] = recommendation.value;
        }
        recommendation.destroy();
        task.save();
      });

      return {};
    }
  );

  //#region Variables

  this.get('/vars', function (schema, { queryParams: { namespace, prefix } }) {
    if (prefix === 'nomad/job-templates') {
      return schema.variables
        .all()
        .filter((v) => v.path.includes('nomad/job-templates'));
    }
    if (namespace && namespace !== '*') {
      return schema.variables.all().filter((v) => v.namespace === namespace);
    } else {
      return schema.variables.all();
    }
  });

  this.get('/var/:id', function ({ variables }, { params }) {
    let variable = variables.find(params.id);
    if (!variable) {
      return new Response(404, {}, {});
    }
    return variable;
  });

  this.put('/var/:id', function (schema, request) {
    const { Path, Namespace, Items } = JSON.parse(request.requestBody);
    if (request.url.includes('cas=') && Path === 'Auto-conflicting Variable') {
      return new Response(
        409,
        {},
        {
          CreateIndex: 65,
          CreateTime: faker.date.recent(14) * 1000000, // in the past couple weeks
          Items: { edited_by: 'your_remote_pal' },
          ModifyIndex: 2118,
          ModifyTime: faker.date.recent(0.01) * 1000000, // a few minutes ago
          Namespace: Namespace,
          Path: Path,
        }
      );
    } else {
      return server.create('variable', {
        path: Path,
        namespace: Namespace,
        items: Items,
        id: Path,
      });
    }
  });

  this.delete('/var/:id', function (schema, request) {
    const { id } = request.params;
    server.db.variables.remove(id);
    return '';
  });

  //#endregion Variables

  //#region Services

  const allocationServiceChecksHandler = function (schema) {
    let disasters = [
      "Moon's haunted",
      'reticulating splines',
      'The operation completed unexpectedly',
      'Ran out of sriracha :(',
      '¯\\_(ツ)_/¯',
      '<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN"\n        "http://www.w3.org/TR/html4/strict.dtd">\n<html>\n    <head>\n        <meta http-equiv="Content-Type" content="text/html;charset=utf-8">\n        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n        <p>Error code: 404</p>\n        <p>Message: File not found.</p>\n        <p>Error code explanation: HTTPStatus.NOT_FOUND - Nothing matches the given URI.</p>\n    </body>\n</html>\n',
    ];
    let fakeChecks = [];
    schema.serviceFragments.all().models.forEach((frag, iter) => {
      [...Array(iter)].forEach((check, checkIter) => {
        const checkOK = faker.random.boolean();
        fakeChecks.push({
          Check: `check-${checkIter}`,
          Group: `job-name.${frag.taskGroup?.name}[1]`,
          Output: checkOK
            ? 'nomad: http ok'
            : disasters[Math.floor(Math.random() * disasters.length)],
          Service: frag.name,
          Status: checkOK ? 'success' : 'failure',
          StatusCode: checkOK ? 200 : 400,
          Task: frag.task?.name,
          Timestamp: new Date().getTime(),
        });
      });
    });
    return fakeChecks;
  };

  this.get('/job/:id/services', function (schema, { params }) {
    const { services } = schema;
    return this.serialize(services.where({ jobId: params.id }));
  });

  this.get('/client/allocation/:id/checks', allocationServiceChecksHandler);

  //#endregion Services

  //#region SSO
  this.get('/acl/auth-methods', function (schema, request) {
    return schema.authMethods.all();
  });
  this.post('/acl/oidc/auth-url', (schema, req) => {
    const { AuthMethodName, ClientNonce, RedirectUri, Meta } = JSON.parse(
      req.requestBody
    );
    return new Response(
      200,
      {},
      {
        AuthURL: `/ui/oidc-mock?auth_method=${AuthMethodName}&client_nonce=${ClientNonce}&redirect_uri=${RedirectUri}&meta=${Meta}`,
      }
    );
  });

  // Simulate an OIDC callback by assuming the code passed is the secret of an existing token, and return that token.
  this.post(
    '/acl/oidc/complete-auth',
    function (schema, req) {
      const code = JSON.parse(req.requestBody).Code;
      const token = schema.tokens.findBy({
        id: code,
      });

      return new Response(
        200,
        {},
        {
          SecretID: token.secretId,
        }
      );
    },
    { timing: 1000 }
  );

  //#endregion SSO
}

function filterKeys(object, ...keys) {
  const clone = copy(object, true);

  keys.forEach((key) => {
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
  if (taskGroupsFromSpec && taskGroupsFromSpec.length)
    tgNames = taskGroupsFromSpec;
  if (taskGroups && taskGroups.length) tgNames = taskGroups;

  return tgNames.reduce((hash, tgName) => {
    hash[tgName] = generateTaskGroupFailures();
    return hash;
  }, {});
}

function generateWarnings() {
  return '2 warnings:\n\n* Group "yourtask" has warnings: 1 error occurred:\n\t* Task "yourtask" has warnings: 1 error occurred:\n\t* 2 errors occurred:\n\t* Identity[vault_default] identities without an audience are insecure\n\t* Identity[vault_default] identities without an expiration are insecure\n* Task yourtask has an identity called vault_default but no vault block';
}
