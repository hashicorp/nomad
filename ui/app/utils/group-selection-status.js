/**
 * Copyright IBM Corp. 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

const terminal = ['complete', 'failed', 'lost'];
const activeDeployments = [
  'running',
  'paused',
  'blocked',
  'unblocking',
  'initializing',
  'pending',
];

// Detail pages load the job, allocations, and deployment independently. Derive
// the same accepted choices reported by /jobs/statuses for the jobs list.
export function groupSelectionStatuses(job, allocations, deployment) {
  const statuses = {};
  const groups = job.taskGroups?.toArray?.() || job.taskGroups || [];
  const tracking =
    activeDeployments.includes(deployment?.status) &&
    deployment?.versionNumber === job.version;

  for (const selection of job.groupSelections || []) {
    const status = {
      Count: selection.Count,
      Groups: selection.Groups,
      Slots: {},
      Unplaced: selection.Count,
    };
    statuses[selection.Name] = status;
    const isMember = (group) =>
      selection.Groups.includes(group) &&
      groups.some(
        (candidate) => candidate.name === group && candidate.count > 0,
      );

    if (tracking) {
      const slots = deployment.groupSelections?.[selection.Name]?.Slots || {};
      for (const [index, slot] of Object.entries(slots)) {
        if (
          Number(index) >= 0 &&
          Number(index) < selection.Count &&
          slot?.Cohort &&
          isMember(slot.TaskGroup)
        ) {
          status.Slots[index] = {
            TaskGroup: slot.TaskGroup,
            Cohort: slot.Cohort,
          };
        }
      }
    } else {
      const incumbents = {};
      for (const alloc of allocations) {
        const claim = alloc.groupSelection;
        if (
          !claim ||
          claim.Name !== selection.Name ||
          claim.Slot < 0 ||
          claim.Slot >= selection.Count ||
          !claim.Cohort ||
          (job.createIndex &&
            alloc.createIndex &&
            alloc.createIndex < job.createIndex) ||
          ['stop', 'evict'].includes(alloc.desiredStatus) ||
          alloc.isCanary ||
          !isMember(alloc.taskGroupName)
        ) {
          continue;
        }
        const previous = incumbents[claim.Slot];
        const isTerminal = terminal.includes(alloc.clientStatus);
        const wasTerminal = terminal.includes(previous?.clientStatus);
        if (
          !previous ||
          (wasTerminal && !isTerminal) ||
          (wasTerminal === isTerminal &&
            (alloc.jobVersion > previous.jobVersion ||
              (alloc.jobVersion === previous.jobVersion &&
                alloc.createIndex > previous.createIndex)))
        ) {
          incumbents[claim.Slot] = alloc;
        }
      }
      for (const [index, alloc] of Object.entries(incumbents)) {
        status.Slots[index] = {
          TaskGroup: alloc.taskGroupName,
          Cohort: alloc.groupSelection.Cohort,
        };
      }
    }
    status.Unplaced -= Object.keys(status.Slots).length;
  }
  return statuses;
}

export function allocationMatchesGroupSelections(alloc, statuses) {
  const selections = Object.values(statuses || {});
  const member = selections.some((selection) =>
    selection.Groups.includes(alloc.taskGroupName),
  );
  if (!member) return true;
  if (['stop', 'evict'].includes(alloc.desiredStatus)) return false;

  const claim = alloc.groupSelection;
  const slot = statuses?.[claim?.Name]?.Slots?.[claim?.Slot];
  return Boolean(
    slot &&
    slot.TaskGroup === alloc.taskGroupName &&
    slot.Cohort === claim.Cohort,
  );
}

export function groupSelectionAllocationCount(groups, statuses) {
  const selections = Object.values(statuses);
  const selected = new Set(
    selections.flatMap((selection) =>
      Object.values(selection.Slots).map((slot) => slot.TaskGroup),
    ),
  );
  return groups.reduce((count, group) => {
    const member = selections.some((selection) =>
      selection.Groups.includes(group.name),
    );
    return count + (!member || selected.has(group.name) ? group.count : 0);
  }, 0);
}
