import { get } from '@ember/object';
import RSVP from 'rsvp';
import { task } from 'ember-concurrency';
import wait from 'nomad-ui/utils/wait';

export function watchRecord(modelName) {
  return task(function*(id) {
    id = get(id, 'id') || id;
    while (true) {
      try {
        yield RSVP.all([
          this.store.findRecord(modelName, id, { reload: true, adapterOptions: { watch: true } }),
          wait(2000),
        ]);
      } catch (e) {
        yield e;
        break;
      }
    }
  });
}

export function watchRelationship(staticRelationshipName) {
  return task(function*(model, throttle = 2000) {
    while (true) {
      try {
        yield RSVP.all([
          this.get('store')
            .adapterFor(model.constructor.modelName)
            .reloadRelationship(model, staticRelationshipName, true),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      }
    }
  });
}

export function watchAll(modelName) {
  return task(function*(throttle = 2000) {
    while (true) {
      try {
        yield RSVP.all([
          this.get('store').findAll(modelName, { reload: true, adapterOptions: { watch: true } }),
          wait(throttle),
        ]);
      } catch (e) {
        yield e;
        break;
      }
    }
  });
}
