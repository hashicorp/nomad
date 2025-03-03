/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import { get, set } from '@ember/object';
import { capitalize } from '@ember/string';
import classic from 'ember-classic-decorator';

@classic
export default class DynamicHostVolumeSerializer extends ApplicationSerializer {
  embeddedRelationships = ['allocations'];

  // Volumes treat Allocations as embedded records. Ember has an
  // EmbeddedRecords mixin, but it assumes an application is using
  // the REST serializer and Nomad does not.
  normalize(typeHash, hash) {
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

  extractEmbeddedRecords(serializer, store, typeHash, partial) {
    partial.included = partial.included || [];

    this.embeddedRelationships.forEach((embed) => {
      const relationshipMeta = typeHash.relationshipsByName.get(embed);
      const relationship = get(partial, `data.relationships.${embed}.data`);

      if (!relationship) return;

      const hasMany = new Array(relationship.length);

      relationship.forEach((alloc, idx) => {
        const { data, included } = this.normalizeEmbeddedRelationship(
          store,
          relationshipMeta,
          alloc
        );

        partial.included.push(data);
        if (included) {
          partial.included.push(...included);
        }

        // In JSONAPI, the main payload value is an array of IDs that
        // map onto the objects in the included array.
        hasMany[idx] = { id: data.id, type: data.type };
      });

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
