import Ember from 'ember';
import { get } from '@ember/object';
import RSVP from 'rsvp';
import { task } from 'ember-concurrency';
import wait from 'nomad-ui/utils/wait';

export function watchRecord(modelName) {
  return task(function*(id, throttle = 2000) {
    if (typeof id === 'object') {
      id = get(id, 'id');
    }
    while (!Ember.testing) {
      try {
        yield RSVP.all([
          this.get('store').findRecord(modelName, id, {
            reload: true,
            adapterOptions: { watch: true },
          }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        this.get('store')
          .adapterFor(modelName)
          .cancelFindRecord(modelName, id);
      }
    }
  }).drop();
}

export function watchRelationship(relationshipName) {
  return task(function*(model, throttle = 2000) {
    while (!Ember.testing) {
      try {
        yield RSVP.all([
          this.get('store')
            .adapterFor(model.constructor.modelName)
            .reloadRelationship(model, relationshipName, true),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        this.get('store')
          .adapterFor(model.constructor.modelName)
          .cancelReloadRelationship(model, relationshipName);
      }
    }
  }).drop();
}

export function watchAll(modelName) {
  return task(function*(throttle = 2000) {
    while (!Ember.testing) {
      try {
        yield RSVP.all([
          this.get('store').findAll(modelName, { reload: true, adapterOptions: { watch: true } }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        this.get('store')
          .adapterFor(modelName)
          .cancelFindAll(modelName);
      }
    }
  }).drop();
}
