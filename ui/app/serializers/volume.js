import { set, get } from '@ember/object';
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  attrs: {
    externalId: 'ExternalID',
  },

  // Volumes treat Allocations as embedded records. Ember has an
  // EmbeddedRecords mixin, but it assumes an application is using
  // the REST serializer and Nomad does not.
  normalize(typeHash, hash) {
    hash.NamespaceID = hash.Namespace;

    hash.PlainId = hash.ID;

    // TODO These shouldn't hardcode `csi/` as part of the IDs,
    // but it is necessary to make the correct find requests and the
    // payload does not contain the required information to derive
    // this identifier.
    hash.ID = JSON.stringify([`csi/${hash.ID}`, hash.NamespaceID || 'default']);
    hash.PluginID = `csi/${hash.PluginID}`;

    const normalizedHash = this._super(typeHash, hash);
    return this.extractEmbeddedRecords(this, this.store, typeHash, normalizedHash);
  },

  keyForRelationship(attr, relationshipType) {
    // Since allocations is embedded, the relationship doesn't map to
    // the expected `AllocationIDs
    if (attr === 'allocations') return 'Allocations';
    return this._super(attr, relationshipType);
  },

  // Convert the embedded Allocations array into JSONAPI included records
  extractEmbeddedRecords(serializer, store, typeHash, partial) {
    const allocationRelationshipMeta = typeHash.relationshipsByName.get('allocations');
    const allocationsRelationship = get(partial, 'data.relationships.allocations.data');

    console.log('Embarking?', allocationRelationshipMeta, partial);
    if (!allocationsRelationship) return partial;

    partial.included = partial.included || [];

    // Create a sidecar relationships array
    const hasMany = new Array(allocationsRelationship.length);

    // For each embedded allocation, normalize the allocation JSON according
    // to the allocation serializer.
    allocationsRelationship.forEach((alloc, idx) => {
      const { data, included } = this.normalizeEmbeddedRelationship(
        store,
        allocationRelationshipMeta,
        alloc
      );

      // In JSONAPI, embedded records go in the included array.
      partial.included.push(data);
      if (included) {
        partial.included.push(...included);
      }

      // In JSONAPI, the main payload value is an array of IDs that
      // map onto the objects in the included array.
      hasMany[idx] = { id: data.id, type: data.type };
    });

    // Set the JSONAPI relationship value to the sidecar.
    const relationship = { data: hasMany };
    set(partial, 'data.relationships.allocations', relationship);

    console.log('Embedding complete');
    console.log(partial);
    return partial;
  },

  normalizeEmbeddedRelationship(store, relationshipMeta, relationshipHash) {
    const modelName = relationshipMeta.type;
    const modelClass = store.modelFor(modelName);
    const serializer = store.serializerFor(modelName);

    return serializer.normalize(modelClass, relationshipHash, null);
  },
});
