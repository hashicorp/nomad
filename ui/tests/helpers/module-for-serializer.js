import Ember from 'ember';
import { moduleForModel } from 'ember-qunit';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

const { getOwner } = Ember;

export default function(modelName, description, options = { needs: [] }) {
  // moduleForModel correctly wires up #Serializer.store,
  // but module does not.
  moduleForModel(modelName, description, {
    unit: true,
    needs: options.needs,
    beforeEach() {
      const model = this.subject();

      // Initializers don't run automatically in unit tests
      fragmentSerializerInitializer(getOwner(model));

      // Reassign the subject to provide the serializer
      this.subject = () => model.store.serializerFor(modelName);

      if (options.beforeEach) {
        options.beforeEach.apply(this, arguments);
      }
    },
    afterEach() {
      if (options.beforeEach) {
        options.beforeEach.apply(this, arguments);
      }
    },
  });
}
