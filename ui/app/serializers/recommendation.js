/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assign } from '@ember/polyfills';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';
import queryString from 'query-string';

@classic
export default class RecommendationSerializer extends ApplicationSerializer {
  attrs = {
    taskName: 'Task',
  };

  separateNanos = ['SubmitTime'];

  extractRelationships(modelClass, hash) {
    const namespace =
      !hash.Namespace || hash.Namespace === 'default'
        ? undefined
        : hash.Namespace;

    const [jobURL] = this.store
      .adapterFor('job')
      .buildURL('job', JSON.stringify([hash.JobID]), hash, 'findRecord')
      .split('?');

    return assign(super.extractRelationships(...arguments), {
      job: {
        links: {
          related: buildURL(jobURL, { namespace }),
        },
      },
    });
  }
}

function buildURL(path, queryParams) {
  const qpString = queryString.stringify(queryParams);
  if (qpString) {
    return `${path}?${qpString}`;
  }
  return path;
}
