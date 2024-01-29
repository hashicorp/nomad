/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import WatchableNamespaceIDs from './watchable-namespace-ids';
import addToPath from 'nomad-ui/utils/add-to-path';
import { base64EncodeString } from 'nomad-ui/utils/encode';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { get } from '@ember/object';
import queryString from 'query-string';

@classic
export default class JobAdapter extends WatchableNamespaceIDs {
  @service system;
  @service watchList;

  relationshipFallbackLinks = {
    summary: '/summary',
  };

  fetchRawDefinition(job) {
    const url = this.urlForFindRecord(job.get('id'), 'job');
    return this.ajax(url, 'GET');
  }

  fetchRawSpecification(job) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job', null, 'submission'),
      '',
      'version=' + job.get('version')
    );
    return this.ajax(url, 'GET');
  }

  forcePeriodic(job) {
    if (job.get('periodic')) {
      const url = addToPath(
        this.urlForFindRecord(job.get('id'), 'job'),
        '/periodic/force'
      );
      return this.ajax(url, 'POST');
    }
  }

  stop(job) {
    const url = this.urlForFindRecord(job.get('id'), 'job');
    return this.ajax(url, 'DELETE');
  }

  purge(job) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job'),
      '',
      'purge=true'
    );

    return this.ajax(url, 'DELETE');
  }

  parse(spec, jobVars) {
    const url = addToPath(this.urlForFindAll('job'), '/parse?namespace=*');
    return this.ajax(url, 'POST', {
      data: {
        JobHCL: spec,
        Variables: jobVars,
        Canonicalize: true,
      },
    });
  }

  plan(job) {
    const jobId = job.get('id') || job.get('_idBeforeSaving');
    const store = this.store;
    const url = addToPath(this.urlForFindRecord(jobId, 'job'), '/plan');

    return this.ajax(url, 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
        Diff: true,
      },
    }).then((json) => {
      json.ID = jobId;
      store.pushPayload('job-plan', { jobPlans: [json] });
      return store.peekRecord('job-plan', jobId);
    });
  }

  // Running a job doesn't follow REST create semantics so it's easier to
  // treat it as an action.
  run(job) {
    let Submission;
    try {
      JSON.parse(job.get('_newDefinition'));
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'json',
      };
    } catch {
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'hcl2',
        Variables: job.get('_newDefinitionVariables'),
      };
    }

    return this.ajax(this.urlForCreateRecord('job'), 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
        Submission,
      },
    });
  }

  update(job) {
    const jobId = job.get('id') || job.get('_idBeforeSaving');

    let Submission;
    try {
      JSON.parse(job.get('_newDefinition'));
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'json',
      };
    } catch {
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'hcl2',
        Variables: job.get('_newDefinitionVariables'),
      };
    }

    return this.ajax(this.urlForUpdateRecord(jobId, 'job'), 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
        Submission,
      },
    });
  }

  scale(job, group, count, message) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job'),
      '/scale'
    );
    return this.ajax(url, 'POST', {
      data: {
        Count: count,
        Message: message,
        Target: {
          Group: group,
        },
        Meta: {
          Source: 'nomad-ui',
        },
      },
    });
  }

  dispatch(job, meta, payload) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job'),
      '/dispatch'
    );
    return this.ajax(url, 'POST', {
      data: {
        Payload: base64EncodeString(payload),
        Meta: meta,
      },
    });
  }

  /**
   *
   * @param {import('../models/job').default} job
   * @param {import('../models/action').default} action
   * @param {string} allocID
   * @returns {string}
   */
  getActionSocketUrl(job, action, allocID) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const region = this.system.activeRegion;

    /**
     * @type {Partial<import('../adapters/application').default>}
     */
    const applicationAdapter = getOwner(this).lookup('adapter:application');
    const prefix = `${
      applicationAdapter.host || window.location.host
    }/${applicationAdapter.urlPrefix()}`;

    const wsUrl =
      `${protocol}//${prefix}/job/${encodeURIComponent(
        job.get('name')
      )}/action` +
      `?namespace=${job.get('namespace.id')}&action=${
        action.name
      }&allocID=${allocID}&task=${
        action.task.name
      }&tty=true&ws_handshake=true` +
      (region ? `&region=${region}` : '');

    return wsUrl;
  }

  query(store, type, query, snapshotRecordArray, options) {
    // let { queryType } = query;
    options = options || {};
    options.adapterOptions = options.adapterOptions || {};

    // const url = this.buildURL(type.modelName, null, null, 'query', query);
    const method = get(options, 'adapterOptions.method') || 'GET';
    const url = this.urlForQuery(query, type.modelName, method);
    console.log('url, method', url, method, options);

    // if (queryType === 'initialize') {
    // //   // options.url = this.urlForQuery(query, type.modelName);
    //   options.adapterOptions.method = 'GET';
    // } else {
    //   options.adapterOptions.watch = true;
    // }
    // if (queryType === 'update') {
    //   options.adapterOptions.method = 'POST';
    //   options.adapterOptions.watch = true; // TODO: probably?
    //   delete query.queryType;
    // }

    // Let's establish the index, via watchList.getIndexFor.

    // url needs to have stringified params on it

    let index = this.watchList.getIndexFor(url);

    // In the case that this is of queryType update,
    // and its index is found to be 1,
    // check for the initialize query's index and use that instead
    if (options.adapterOptions.queryType === 'update' && index === 1) {
      let initializeQueryIndex = this.watchList.getIndexFor(
        '/v1/jobs/statuses3?meta=true&per_page=10'
      ); // TODO: fickle!
      if (initializeQueryIndex) {
        console.log('initializeQUeryIndex', initializeQueryIndex);
        index = initializeQueryIndex;
      }
    }

    // TODO: adding a new job hash will not necessarily cancel the old one.
    // You could be holding open a POST on jobs AB and ABC at the same time.

    console.log('index for', url, 'is', index);
    if (this.watchList.getIndexFor(url)) {
      query.index = index;
    }

    // console.log('so then uh', query);
    // } else if (queryType === 'update') {
    //   // options.url = this.urlForUpdateQuery(query, type.modelName);
    //   options.adapterOptions.method = 'POST';
    //   options.adapterOptions.watch = true;
    // }
    // return super.query(store, type, query, snapshotRecordArray, options);
    // let superQuery = super.query(store, type, query, snapshotRecordArray, options);
    // console.log('superquery', superQuery);
    // return superQuery;

    const signal = get(options, 'adapterOptions.abortController.signal');
    return this.ajax(url, method, {
      signal,
      data: query,
      skipURLModification: true,
    });
  }

  handleResponse(status, headers, payload, requestData) {
    // console.log('jobadapter handleResponse', status, headers, payload, requestData);
    /**
     * @type {Object}
     */
    const result = super.handleResponse(...arguments);
    // console.log('response', result, headers);
    if (result) {
      result.meta = result.meta || {};
      if (headers['x-nomad-nexttoken']) {
        result.meta.nextToken = headers['x-nomad-nexttoken'];
      }
    }

    // If the url contains the urlForQuery, we should fire a new method that handles index tracking
    if (requestData.url.includes(this.urlForQuery())) {
      this.updateQueryIndex(headers['x-nomad-index']);
    }

    return result;
  }

  // urlForQuery(query, modelName) {
  //   return `/${this.namespace}/jobs/statuses3`;
  // }

  urlForQuery(query, modelName, method) {
    let baseUrl = `/${this.namespace}/jobs/statuses3`;
    // let queryString = Object.keys(query).map(key => `${encodeURIComponent(key)}=${encodeURIComponent(query[key])}`).join('&');
    if (method === 'POST') {
      return `${baseUrl}?hash=${btoa(JSON.stringify(query))}`;
    } else {
      return `${baseUrl}?${queryString.stringify(query)}`;
    }
  }

  // urlForQuery(query, modelName) {
  //   let baseUrl = `/${this.namespace}/jobs/statuses3`;
  //   // let queryString = Object.keys(query).map(key => `${encodeURIComponent(key)}=${encodeURIComponent(query[key])}`).join('&');
  //   // Only include non-empty query params
  //   let queryString = Object.keys(query).filter(key => !!query[key]).map(key => `${encodeURIComponent(key)}=${encodeURIComponent(query[key])}`).join('&');
  //   console.log('+++ querystring', queryString)
  //   return `${baseUrl}?${queryString}`;
  // }

  ajaxOptions(url, type, options) {
    let hash = super.ajaxOptions(url, type, options);
    console.log('+++ ajaxOptions', url, type, options, hash);
    // debugger;
    // console.log('options', options, hash);

    // Custom handling for POST requests to append 'index' as a query parameter
    if (type === 'POST' && options.data && options.data.index) {
      let index = encodeURIComponent(options.data.index);
      hash.url = `${hash.url}&index=${index}`;
    }

    return hash;
  }

  updateQueryIndex(index) {
    console.log('setQueryIndex', index);
    // Is there an established index for jobs
    // this.watchList.setIndexFor(url, index);
  }
}
