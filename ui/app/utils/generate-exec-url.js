/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';

export default function generateExecUrl(
  router,
  { job, taskGroup, task, allocation }
) {
  const queryParams = {};

  const namespace = get(job, 'namespace.name');
  const region = get(job, 'region');

  if (namespace) {
    queryParams.namespace = namespace;
  }

  if (region) {
    queryParams.region = region;
  }

  if (task) {
    const queryParamsOptions = {
      ...queryParams,
    };

    if (allocation) {
      queryParamsOptions.allocation = get(allocation, 'shortId');
    }

    return router.urlFor(
      'exec.task-group.task',
      get(job, 'plainId'),
      get(taskGroup, 'name'),
      get(task, 'name'),
      {
        queryParams: queryParamsOptions,
      }
    );
  } else if (taskGroup) {
    return router.urlFor(
      'exec.task-group',
      get(job, 'plainId'),
      get(taskGroup, 'name'),
      {
        queryParams,
      }
    );
  } else if (allocation) {
    if (get(allocation, 'taskGroup.tasks.length') === 1) {
      return router.urlFor(
        'exec.task-group.task',
        get(job, 'plainId'),
        get(allocation, 'taskGroup.name'),
        get(allocation, 'taskGroup.tasks.firstObject.name'),
        {
          queryParams: {
            allocation: get(allocation, 'shortId'),
            ...queryParams,
          },
        }
      );
    } else {
      return router.urlFor(
        'exec.task-group',
        get(job, 'plainId'),
        get(allocation, 'taskGroup.name'),
        {
          queryParams: {
            allocation: get(allocation, 'shortId'),
            ...queryParams,
          },
        }
      );
    }
  } else {
    return router.urlFor('exec', get(job, 'plainId'), { queryParams });
  }
}
