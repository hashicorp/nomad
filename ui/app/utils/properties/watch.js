import Ember from 'ember';
import { get } from '@ember/object';
import RSVP from 'rsvp';
import { task } from 'ember-concurrency';
import wait from 'nomad-ui/utils/wait';
import XHRToken from 'nomad-ui/utils/classes/xhr-token';
import config from 'nomad-ui/config/environment';

const isEnabled = config.APP.blockingQueries !== false;

export function watchRecord(modelName, { ignore404 } = {}) {
  let watchRecordTask = task(function*(id, { throttle = 2000, runInTests = false } = {}) {
    const token = new XHRToken();
    if (typeof id === 'object') {
      id = get(id, 'id');
    }

    while (isEnabled && (!Ember.testing || runInTests)) {
      try {
        yield RSVP.all([
          this.store.findRecord(modelName, id, {
            reload: true,
            adapterOptions: { watch: true, abortToken: token },
          }),
          wait(throttle),
        ]);
      } catch (e) {
        if (!ignore404) {
        }

        throw { error: e, modelName };
      } finally {
        token.abort();
      }
    }
  }).drop();

  if (ignore404) {
    return watchRecordTask;
  } else {
    return watchRecordTask.evented();
  }
}

export function watchRelationship(relationshipName) {
  return task(function*(model, throttle = 2000) {
    const token = new XHRToken();
    while (isEnabled && !Ember.testing) {
      try {
        yield RSVP.all([
          this.store
            .adapterFor(model.constructor.modelName)
            .reloadRelationship(model, relationshipName, { watch: true, abortToken: token }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        token.abort();
      }
    }
  }).drop();
}

export function watchAll(modelName) {
  return task(function*(throttle = 2000) {
    const token = new XHRToken();
    while (isEnabled && !Ember.testing) {
      try {
        yield RSVP.all([
          this.store.findAll(modelName, {
            reload: true,
            adapterOptions: { watch: true, abortToken: token },
          }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      } finally {
        token.abort();
      }
    }
  }).drop();
}
