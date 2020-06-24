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

export function watchRecord(modelName) {
  return task(function*(id, throttle = 2000) {
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
        yield e;
        break;
      } finally {
        controller.abort();
      }
    }
  }).drop();
}

export function watchRelationship(relationshipName) {
  return task(function*(model, throttle = 2000) {
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

export function watchAll(modelName) {
  return task(function*(throttle = 2000) {
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
  return task(function*(params, throttle = 10000) {
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
