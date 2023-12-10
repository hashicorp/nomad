/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | abstract', function (hooks) {
  setupTest(hooks);
  setupAbility('abstract')(hooks);
  hooks.beforeEach(function () {
    const mockSystem = Service.extend({
      init(...args) {
        this._super(...args);

        this.features = this.features || [];
      },
    });

    this.owner.register('service:system', mockSystem);
  });

  module('#_findMatchingNamespace', function () {
    test('it returns * if no matching namespace and * is specified', function (assert) {
      const policyNamespaces = [
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'default',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
              {
                Capabilities: [],
                PathSpec: 'pablo/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: '*',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'madness',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['read', 'list', 'write'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
      ];

      assert.equal(
        this.ability._findMatchingNamespace(policyNamespaces, 'pablo'),
        '*'
      );
    });

    test('it returns the matching namespace if it exists', function (assert) {
      const policyNamespaces = [
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'default',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
              {
                Capabilities: [],
                PathSpec: 'pablo/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: '*',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'pablo',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['read', 'list', 'write'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
      ];

      assert.equal(
        this.ability._findMatchingNamespace(policyNamespaces, 'pablo'),
        'pablo'
      );
    });

    test('it handles glob matching suffix', function (assert) {
      const policyNamespaces = [
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'default',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
              {
                Capabilities: [],
                PathSpec: 'pablo/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: '*',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'pablo/*',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['read', 'list', 'write'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
      ];

      assert.equal(
        this.ability._findMatchingNamespace(
          policyNamespaces,
          'pablo/picasso/rothkos/rilkes'
        ),
        'pablo/*'
      );
    });

    test('it handles glob matching prefix', function (assert) {
      const policyNamespaces = [
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'default',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
              {
                Capabilities: [],
                PathSpec: 'pablo/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: '*',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: '*/rilkes',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['read', 'list', 'write'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
      ];

      assert.equal(
        this.ability._findMatchingNamespace(
          policyNamespaces,
          'pablo/picasso/rothkos/rilkes'
        ),
        '*/rilkes'
      );
    });

    test('it returns default if no matching namespace and no matching globs', function (assert) {
      const policyNamespaces = [
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'default',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: 'project/*',
              },
              {
                Capabilities: ['write', 'read', 'destroy', 'list'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
              {
                Capabilities: [],
                PathSpec: 'pablo/*',
              },
            ],
          },
        },
        {
          Capabilities: [
            'list-jobs',
            'parse-job',
            'read-job',
            'csi-list-volume',
            'csi-read-volume',
            'read-job-scaling',
            'list-scaling-policies',
            'read-scaling-policy',
            'scale-job',
            'submit-job',
            'dispatch-job',
            'read-logs',
            'read-fs',
            'alloc-exec',
            'alloc-lifecycle',
            'csi-mount-volume',
            'csi-write-volume',
            'submit-recommendation',
          ],
          Name: 'pablo/*',
          Policy: 'write',
          Variables: {
            Paths: [
              {
                Capabilities: ['read', 'list', 'write'],
                PathSpec: '*',
              },
              {
                Capabilities: ['read', 'list'],
                PathSpec: 'system/*',
              },
            ],
          },
        },
      ];

      assert.equal(
        this.ability._findMatchingNamespace(policyNamespaces, 'carter'),
        'default'
      );
    });
  });
});
