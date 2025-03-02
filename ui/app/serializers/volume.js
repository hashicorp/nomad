/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { set, get } from '@ember/object';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';
import { capitalize } from '@ember/string';
@classic
export default class VolumeSerializer extends ApplicationSerializer {
  attrs = {
    externalId: 'ExternalID',
  };

  embeddedRelationships = ['writeAllocations', 'readAllocations'];

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

    // Populate read/write allocation lists from aggregate allocation list
    const readAllocs = hash.ReadAllocs || {};
    const writeAllocs = hash.WriteAllocs || {};

    hash.ReadAllocations = [];
    hash.WriteAllocations = [];

    if (hash.Allocations) {
      hash.Allocations.forEach(function (alloc) {
        const id = alloc.ID;
        if (id in readAllocs) {
          hash.ReadAllocations.push(alloc);
        }
        if (id in writeAllocs) {
          hash.WriteAllocations.push(alloc);
        }
      });
      delete hash.Allocations;
    }

    const normalizedHash = super.normalize(typeHash, hash);
    return this.extractEmbeddedRecords(
      this,
      this.store,
      typeHash,
      normalizedHash
    );
  }

  keyForRelationship(attr, relationshipType) {
    //Embedded relationship attributes don't end in IDs
    if (this.embeddedRelationships.includes(attr)) return capitalize(attr);
    return super.keyForRelationship(attr, relationshipType);
  }

  // Convert the embedded relationship arrays into JSONAPI included records
  extractEmbeddedRecords(serializer, store, typeHash, partial) {
    partial.included = partial.included || [];

    this.embeddedRelationships.forEach((embed) => {
      const relationshipMeta = typeHash.relationshipsByName.get(embed);
      const relationship = get(partial, `data.relationships.${embed}.data`);

      if (!relationship) return;

      // Create a sidecar relationships array
      const hasMany = new Array(relationship.length);

      // For each embedded allocation, normalize the allocation JSON according
      // to the allocation serializer.
      relationship.forEach((alloc, idx) => {
        const { data, included } = this.normalizeEmbeddedRelationship(
          store,
          relationshipMeta,
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
      const relationshipJson = { data: hasMany };
      set(partial, `data.relationships.${embed}`, relationshipJson);
    });

    return partial;
  }

  normalizeEmbeddedRelationship(store, relationshipMeta, relationshipHash) {
    const modelName = relationshipMeta.type;
    const modelClass = store.modelFor(modelName);
    const serializer = store.serializerFor(modelName);

    return serializer.normalize(modelClass, relationshipHash, null);
  }
}
