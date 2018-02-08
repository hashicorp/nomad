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
  return task(function*(model, relationshipName) {
    while (true) {
      try {
        yield RSVP.all([
          this.store
            .adapterFor(model.get('modelName'))
            .reloadRelationship(model, staticRelationshipName || relationshipName, true),
          wait(2000),
        ]);
      } catch (e) {
        yield e;
        break;
      }
    }
  });
}
