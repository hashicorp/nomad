/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import { alias, equal } from '@ember/object/computed';
import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class VolumeMount extends Fragment {
  @fragmentOwner() task;

  @attr('string') volume;

  @computed('task.taskGroup.volumes.@each.name', 'volume')
  get volumeDeclaration() {
    return this.task.taskGroup.volumes.findBy('name', this.volume);
  }

  @equal('volumeDeclaration.type', 'csi') isCSI;
  @alias('volumeDeclaration.source') source;

  // Since CSI volumes are namespaced, the link intent of a volume mount will
  // be to the CSI volume with a namespace that matches this task's job's namespace.
  @alias('task.taskGroup.job.namespace') namespace;

  @attr('string') destination;
  @attr('string') propagationMode;
  @attr('boolean') readOnly;
}
