/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import { module, test } from 'qunit';
import sinon from 'sinon';

const emptyOptions = { queryParams: {} };

module('Unit | Utility | generate-exec-url', function (hooks) {
  hooks.beforeEach(function () {
    this.urlForSpy = sinon.spy();
    this.router = { urlFor: this.urlForSpy, currentRoute: { queryParams: {} } };
  });

  test('it generates an exec job URL', function (assert) {
    generateExecUrl(this.router, { job: { plainId: 'job-name' } });

    assert.ok(this.urlForSpy.calledWith('exec', 'job-name', emptyOptions));
  });

  test('it generates an exec job URL with an allocation and task group when there are multiple tasks', function (assert) {
    generateExecUrl(this.router, {
      job: { plainId: 'job-name' },
      allocation: {
        shortId: 'allocation-short-id',
        taskGroup: { name: 'task-group-name', tasks: [0, 1, 2] },
      },
    });

    assert.ok(
      this.urlForSpy.calledWith(
        'exec.task-group',
        'job-name',
        'task-group-name',
        {
          queryParams: { allocation: 'allocation-short-id' },
        }
      )
    );
  });

  test('it generates an exec job URL with an allocation, task group, and task when there is only one task', function (assert) {
    generateExecUrl(this.router, {
      job: { plainId: 'job-name' },
      allocation: {
        shortId: 'allocation-short-id',
        taskGroup: { name: 'task-group-name', tasks: [{ name: 'task-name' }] },
      },
    });

    assert.ok(
      this.urlForSpy.calledWith(
        'exec.task-group.task',
        'job-name',
        'task-group-name',
        'task-name',
        {
          queryParams: { allocation: 'allocation-short-id' },
        }
      )
    );
  });

  test('it generates an exec task group URL', function (assert) {
    generateExecUrl(this.router, {
      job: { plainId: 'job-name' },
      taskGroup: { name: 'task-group-name' },
    });

    assert.ok(
      this.urlForSpy.calledWith(
        'exec.task-group',
        'job-name',
        'task-group-name',
        emptyOptions
      )
    );
  });

  test('it generates an exec task URL', function (assert) {
    generateExecUrl(this.router, {
      allocation: { shortId: 'allocation-short-id' },
      job: { plainId: 'job-name' },
      taskGroup: { name: 'task-group-name' },
      task: { name: 'task-name' },
    });

    assert.ok(
      this.urlForSpy.calledWith(
        'exec.task-group.task',
        'job-name',
        'task-group-name',
        'task-name',
        { queryParams: { allocation: 'allocation-short-id' } }
      )
    );
  });

  test('it generates an exec task URL without an allocation', function (assert) {
    generateExecUrl(this.router, {
      job: { plainId: 'job-name' },
      taskGroup: { name: 'task-group-name' },
      task: { name: 'task-name' },
    });

    assert.ok(
      this.urlForSpy.calledWith(
        'exec.task-group.task',
        'job-name',
        'task-group-name',
        'task-name'
      )
    );
  });

  test('it includes job namespace and region when they exist', function (assert) {
    generateExecUrl(this.router, {
      job: {
        namespace: {
          name: 'a-namespace',
        },
        plainId: 'job-name',
        region: 'a-region',
      },
      allocation: {
        shortId: 'id',
        taskGroup: { name: 'task-group-name', tasks: [0, 1] },
      },
    });

    assert.ok(
      this.urlForSpy.calledWith(
        'exec.task-group',
        'job-name',
        'task-group-name',
        {
          queryParams: {
            allocation: 'id',
            namespace: 'a-namespace',
            region: 'a-region',
          },
        }
      )
    );
  });
});
