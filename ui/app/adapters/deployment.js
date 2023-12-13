/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Watchable from './watchable';
import classic from 'ember-classic-decorator';

@classic
export default class DeploymentAdapter extends Watchable {
  fail(deployment) {
    const id = deployment.get('id');
    const url = urlForAction(this.urlForFindRecord(id, 'deployment'), '/fail');
    return this.ajax(url, 'POST', {
      data: {
        DeploymentId: id,
      },
    });
  }

  promote(deployment) {
    const id = deployment.get('id');
    const url = urlForAction(
      this.urlForFindRecord(id, 'deployment'),
      '/promote'
    );
    return this.ajax(url, 'POST', {
      data: {
        DeploymentId: id,
        All: true,
      },
    });
  }
}

// The deployment action API endpoints all end with the ID
// /deployment/:action/:deployment_id instead of /deployment/:deployment_id/:action
function urlForAction(url, extension = '') {
  const [path, params] = url.split('?');
  const pathParts = path.split('/');
  const idPart = pathParts.pop();
  let newUrl = `${pathParts.join('/')}${extension}/${idPart}`;

  if (params) {
    newUrl += `?${params}`;
  }

  return newUrl;
}
