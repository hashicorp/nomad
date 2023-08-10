/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { capitalize } from '@ember/string';

export default helper(function formatTemplateLabel([path]) {
  // Removes the preceeding nomad/job-templates/default/
  let label;
  const delimiter = path.lastIndexOf('/');
  if (delimiter !== -1) {
    label = path.slice(delimiter + 1);
  } else {
    label = path;
  }
  return capitalize(label).split('-').join(' ');
});
