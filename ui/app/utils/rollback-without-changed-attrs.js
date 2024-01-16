/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function rollbackWithoutChangedAttrs(model) {
  // The purpose of this function was to allow deletes to fail
  // and then roll them back without rolling back
  // other changed attributes.

  // A failed delete followed by trying to re-view the
  // model in question was throwing uncaught Errros

  let forLater = {};
  Object.keys(model.changedAttributes()).forEach((key) => {
    forLater[key] = model.get(key);
  });

  model.rollbackAttributes();

  Object.keys(forLater).forEach((key) => {
    model.set(key, forLater[key]);
  });
}
