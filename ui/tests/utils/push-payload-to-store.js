/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { run } from '@ember/runloop';

// These are private store methods called by store "finder" methods.
// Useful in unit tests when there is store interaction, since calling
// adapter and serializer methods directly will never insert data into
// the store.
export default function pushPayloadToStore(store, payload, modelName) {
  run(() => {
    store.push(payload);
    // Simulate the reactive update that findAll would trigger so computed
    // dependencies on peekAll(modelName) re-evaluate in unit tests.
    store.peekAll(modelName).notifyPropertyChange('[]');
  });
}
