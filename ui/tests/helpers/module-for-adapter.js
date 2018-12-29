import { getOwner } from '@ember/application';
import { moduleForModel } from 'ember-qunit';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

export default function(modelName, description, options = { needs: [] }) {
  // moduleForModel correctly creates the store service
  // but moduleFor does not.
  moduleForModel(modelName, description, {
    unit: true,
    needs: options.needs,
    beforeEach() {
      const model = this.subject();

      // Initializers don't run automatically in unit tests
      fragmentSerializerInitializer(getOwner(model));

      // Reassign the subject to provide the adapter
      this.subject = () => model.store.adapterFor(modelName);

      // Expose the store as well, since it is a parameter for many adapter methods
      this.store = model.store;

      if (options.beforeEach) {
        options.beforeEach.apply(this, arguments);
      }
    },
    afterEach() {
      if (options.afterEach) {
        options.afterEach.apply(this, arguments);
      }
    },
  });
}
