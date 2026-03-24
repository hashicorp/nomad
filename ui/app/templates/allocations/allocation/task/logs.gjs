/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import TaskLog from 'nomad-ui/components/task-log';
import TaskSubnav from 'nomad-ui/components/task-subnav';

<template>
  {{pageTitle "Task " @model.name " logs"}}
  <TaskSubnav @task={{@model}} />
  <section class="section is-full-width">
    <TaskLog
      data-test-task-log
      @allocation={{@model.allocation}}
      @task={{@model.name}}
    />
  </section>
</template>
