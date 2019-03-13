import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

export default function(modelName, description, options = { needs: [] }) {
  // moduleForModel correctly wires up #Serializer.store,
  // but module does not.
  module(description, function(hooks) {
    setupTest(hooks);
    this.store = this.owner.lookup('service:store');

    // Initializers don't run automatically in unit tests
    fragmentSerializerInitializer(this.owner);

    // Reassign the subject to provide the serializer
    this.subject = () => this.store.serializerFor(modelName);

    if (options.beforeEach) {
      options.beforeEach.apply(this, arguments);
    }
  });
}
