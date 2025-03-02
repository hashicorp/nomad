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

    this.embeddedRelationships.forEach((relationshipName) => {
      const relationshipMeta =
        typeHash.relationshipsByName.get(relationshipName);
      const relationship = get(
        partial,
        `data.relationships.${relationshipName}.data`
      );

      if (!relationship) return;

      const hasMany = new Array(relationship.length);

      relationship.forEach((alloc, idx) => {
        const { data, included } = this.normalizeEmbeddedRelationship(
          store,
          relationshipMeta,
          alloc
        );

        hasMany[idx] = data;
        if (included) partial.included.push(...included);
      });

      const relationshipJson = { data: hasMany };
      set(partial, `data.relationships.${relationshipName}`, relationshipJson);
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
