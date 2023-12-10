/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { equal } from '@ember/object/computed';
import Component from '@ember/component';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('job-diff')
@classNameBindings(
  'isEdited:is-edited',
  'isAdded:is-added',
  'isDeleted:is-deleted'
)
export default class JobDiff extends Component {
  diff = null;

  verbose = true;

  @equal('diff.Type', 'Edited') isEdited;
  @equal('diff.Type', 'Added') isAdded;
  @equal('diff.Type', 'Deleted') isDeleted;
}
