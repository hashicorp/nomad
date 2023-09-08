/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { camelize, capitalize } from '@ember/string';
import { RestSerializer } from 'ember-cli-mirage';

const keyCase = (str) =>
  str === 'id' ? 'ID' : capitalize(camelize(str)).replace(/Id/g, 'ID');

export default RestSerializer.extend({
  serialize() {
    const json = RestSerializer.prototype.serialize.apply(this, arguments);
    const keys = Object.keys(json);
    if (keys.length === 1) {
      return json[keys[0]];
    } else {
      return json;
    }
  },

  keyForModel: keyCase,
  keyForForeignKey: (str) => `${keyCase(str)}ID`,
  keyForCollection: keyCase,
  keyForAttribute: keyCase,
  keyForRelationship: keyCase,
  keyForRelationshipIds: keyCase,
  keyForEmbeddedRelationship: keyCase,
});
