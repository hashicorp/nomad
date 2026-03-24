/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import ExecTaskGroupParent from 'nomad-ui/components/exec/task-group-parent';
import ExecTerminal from 'nomad-ui/components/exec-terminal';
import NomadLogo from 'nomad-ui/components/nomad-logo';
import { HdsIcon } from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle
    (if
      @controller.currentRegion
      (concat "Exec - " @controller.currentRegion)
      "Exec"
    )
  }}
  <nav class="navbar is-popup">
    <div class="navbar-brand">
      <div class="navbar-item is-logo">
        <NomadLogo />
      </div>
    </div>
    {{#if @controller.system.shouldShowRegions}}
      <div class="navbar-item">
        <span class="navbar-label">Region</span>
        <span data-test-region>{{@controller.currentRegion}}</span>
      </div>
    {{/if}}

    {{#if @controller.system.shouldShowNamespaces}}
      <div class="navbar-item">
        <span class="navbar-label">Namespace</span>
        <span data-test-namespace>{{@controller.displayNamespace}}</span>
      </div>
    {{/if}}

    <div class="navbar-item">
      <span class="navbar-label">Job</span>
      <span data-test-job>{{@controller.displayJobName}}</span>
    </div>
    <div class="navbar-end">
      <a
        href="https://developer.hashicorp.com/nomad/docs"
        target="_blank"
        rel="noopener noreferrer"
        class="navbar-item"
      >Documentation</a>
      <HdsIcon @name="lock" />
    </div>
  </nav>

  {{#if @controller.isJobDead}}
    <div class="exec-window" data-test-exec-job-dead>
      <div class="task-group-tree">
      </div>
      <div class="terminal-container" data-test-exec-job-dead-message>
        Job
        <code>{{@controller.displayJobName}}</code>
        is dead and cannot host an exec session.
      </div>
    </div>
  {{else}}
    <div class="exec-window">
      <div class="task-group-tree">
        <h4 class="title is-6">Tasks</h4>
        <ul>
          {{#each @controller.sortedTaskGroups as |taskGroup|}}
            <li data-test-task-group>
              <ExecTaskGroupParent
                @taskGroup={{taskGroup}}
                @shouldOpenInNewWindow={{@controller.socketOpen}}
                @activeTaskName={{@controller.taskName}}
                @activeTaskGroupName={{@controller.taskGroupName}}
              />
            </li>
          {{/each}}
        </ul>
      </div>
      <ExecTerminal
        @terminal={{@controller.terminal}}
        @socketOpen={{@controller.socketOpen}}
      />
    </div>
  {{/if}}
</template>
