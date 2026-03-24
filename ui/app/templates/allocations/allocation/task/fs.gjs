/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import FsBrowser from 'nomad-ui/components/fs/browser';
import TaskSubnav from 'nomad-ui/components/task-subnav';

<template>
  {{pageTitle
    @controller.pathWithLeadingSlash
    " - Task "
    @controller.taskState.name
    " filesystem"
  }}
  <TaskSubnav @task={{@controller.taskState}} />
  <FsBrowser
    @model={{@controller.taskState}}
    @path={{@controller.path}}
    @stat={{@controller.stat}}
    @isFile={{@controller.isFile}}
    @directoryEntries={{@controller.directoryEntries}}
    @sortProperty={{@controller.sortProperty}}
    @sortDescending={{@controller.sortDescending}}
  />
</template>
