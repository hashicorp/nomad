/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Ember from 'ember';
import { get } from '@ember/object';
import { assert } from '@ember/debug';
import RSVP from 'rsvp';
import { task } from 'ember-concurrency';
import { AbortController } from 'fetch';
import wait from 'nomad-ui/utils/wait';
import Watchable from 'nomad-ui/adapters/watchable';
import config from 'nomad-ui/config/environment';

const isEnabled = config.APP.blockingQueries !== false;

/**
 * @typedef watchRecordOptions
 * @property {boolean} [shouldSurfaceErrors=false] - If true, the task will throw errors instead of yielding them.
 */

/**
 * @param {string} modelName - The name of the model to watch.
 * @param {watchRecordOptions} [options]
 */
export function watchRecord(modelName, { shouldSurfaceErrors = false } = {}) {
  return task(function* (id, throttle = 2000) {
    assert(
      'To watch a record, the record adapter MUST extend Watchable',
      this.store.adapterFor(modelName) instanceof Watchable
    );
    if (typeof id === 'object') {
      id = get(id, 'id');
    }
    while (isEnabled && !Ember.testing) {
      const controller = new AbortController();
      try {
        yield RSVP.all([
          this.store.findRecord(modelName, id, {
            reload: true,
            adapterOptions: { watch: true, abortController: controller },
          }),
          wait(throttle),
        ]);
      } catch (e) {
        if (shouldSurfaceErrors) {
          throw e;
        }
        yield e;
        break;
      } finally {
        controller.abort();
      }
    }
  }).drop();
}

export function watchRelationship(relationshipName, replace = false) {
  return task(function* (model, throttle = 2000) {
    assert(
      'To watch a relationship, the adapter of the model provided to the watchRelationship task MUST extend Watchable',
      this.store.adapterFor(model.constructor.modelName) instanceof Watchable
    );
    while (isEnabled && !Ember.testing) {
      const controller = new AbortController();
      try {
        yield RSVP.all([
          this.store
            .adapterFor(model.constructor.modelName)
            .reloadRelationship(model, relationshipName, {
              watch: true,
              abortController: controller,
              replace,
            }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        controller.abort();
      }
    }
  }).drop();
}

export function watchNonStoreRecords(modelName) {
  return task(function* (model, asyncCallbackName, throttle = 5000) {
    assert(
      'To watch a non-store records, the adapter of the model provided to the watchNonStoreRecords task MUST extend Watchable',
      this.store.adapterFor(modelName) instanceof Watchable
    );
    while (isEnabled && !Ember.testing) {
      const controller = new AbortController();
      try {
        yield model[asyncCallbackName]();
        yield wait(throttle);
      } catch (e) {
        yield e;
        break;
      } finally {
        controller.abort();
      }
    }
  }).drop();
}

export function watchAll(modelName) {
  return task(function* (throttle = 2000) {
    assert(
      'To watch all, the respective adapter MUST extend Watchable',
      this.store.adapterFor(modelName) instanceof Watchable
    );
    while (isEnabled && !Ember.testing) {
      const controller = new AbortController();
      try {
        yield RSVP.all([
          this.store.findAll(modelName, {
            reload: true,
            adapterOptions: { watch: true, abortController: controller },
          }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        controller.abort();
      }
    }
  }).drop();
}

export function watchQuery(modelName) {
  return task(function* (params, throttle = 10000) {
    assert(
      'To watch a query, the adapter for the type being queried MUST extend Watchable',
      this.store.adapterFor(modelName) instanceof Watchable
    );
    while (isEnabled && !Ember.testing) {
      const controller = new AbortController();
      try {
        yield RSVP.all([
          this.store.query(modelName, params, {
            reload: true,
            adapterOptions: { watch: true, abortController: controller },
          }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        controller.abort();
      }
    }
  }).drop();
}
