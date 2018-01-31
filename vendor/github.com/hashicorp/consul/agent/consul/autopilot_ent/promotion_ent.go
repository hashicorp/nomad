package autopilot_ent

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/raft"
)

const UpgradeWarnInterval = 30 * time.Second

// autopilotServer is a structure for consolidating autopilot-relevant server information
// for making decisions about promoting non-voters.
type autopilotServer struct {
	raft.Server
	health             *autopilot.ServerHealth
	version            *version.Version
	designatedNonVoter bool
	redundancyZone     string
}

// AdvancedPromoter handles the advanced promotion logic for supporting enterprise
// features like redundancy zones and upgrade migrations.
type AdvancedPromoter struct {
	logger   *log.Logger
	delegate autopilot.Delegate

	// nodeMetaFunc is a function that returns the node meta tag/values for
	// a given raft server ID.
	nodeMetaFunc func(raft.ServerID) (map[string]string, error)

	// lastUpgradeTagWarning is the time since the last tag warning.
	lastUpgradeTagWarning time.Time
}

func NewAdvancedPromoter(logger *log.Logger, delegate autopilot.Delegate, nodeMetaFunc func(raft.ServerID) (map[string]string, error)) *AdvancedPromoter {
	return &AdvancedPromoter{
		logger:       logger,
		delegate:     delegate,
		nodeMetaFunc: nodeMetaFunc,
	}
}

// PromoteNonVoters returns promotion info, taking into account redundancy and
// non-voter tags, and carries out any demotions required for upgrade migrations.
func (a *AdvancedPromoter) PromoteNonVoters(conf *autopilot.Config, health autopilot.OperatorHealthReply) ([]raft.Server, error) {
	future := a.delegate.Raft().GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to get raft configuration: %v", err)
	}
	raftServers := future.Configuration().Servers

	servers := make(map[raft.ServerID]*autopilotServer)
	zoneVoters := make(map[string]struct{})
	now := time.Now()
	for _, s := range raftServers {
		server := &autopilotServer{}
		server.ID = s.ID
		server.Address = s.Address
		server.Suffrage = s.Suffrage
		server.health = health.ServerHealth(string(s.ID))

		meta, err := a.nodeMetaFunc(s.ID)
		if err != nil {
			a.logger.Printf("[ERR] autopilot: error getting node metadata for server id %q: %s", s.ID, err)
		} else {
			if conf.RedundancyZoneTag != "" {
				zone := meta[conf.RedundancyZoneTag]
				if zone != "" {
					if autopilot.IsPotentialVoter(s.Suffrage) && server.health.IsStable(now, conf) {
						zoneVoters[zone] = struct{}{}
					}
					server.redundancyZone = zone
				}
			}

			if conf.UpgradeVersionTag != "" {
				raw := meta[conf.UpgradeVersionTag]
				version, err := version.NewVersion(raw)
				if err != nil {
					a.logger.Printf("[ERR] autopilot: error parsing version for server id %q: %s", s.ID, err)
				} else {
					server.version = version
				}
			}
		}

		servers[s.ID] = server
	}

	// Construct a view of the servers in the cluster
	for _, member := range a.delegate.Serf().Members() {
		serverInfo, err := a.delegate.IsServer(member)
		if err != nil || serverInfo == nil {
			continue
		}

		server, ok := servers[raft.ServerID(serverInfo.ID)]
		if !ok {
			continue
		}

		_, server.designatedNonVoter = member.Tags["nonvoter"]
		if conf.UpgradeVersionTag == "" {
			server.version = &serverInfo.Build
		}
	}

	// Remove servers with a missing version so it's easier to reason about min/max versions
	// in the cluster
	for id, server := range servers {
		if server.version == nil {
			delete(servers, id)
		}
	}

	// If we have some newer-versioned servers, check to see whether we have enough of
	// them to begin promoting them and demoting old servers. If we don't have enough servers
	// with version info, skip this step.
	var migrationInfo migrationInfo
	if !conf.DisableUpgradeMigration {
		if len(servers) == len(raftServers) {
			migrationInfo = computeMigrationInfo(servers)
		} else if conf.UpgradeVersionTag != "" {
			// If they defined an upgrade tag but didn't set it, log a warning at a throttled rate
			if now.Sub(a.lastUpgradeTagWarning) > UpgradeWarnInterval {
				a.lastUpgradeTagWarning = now
				a.logger.Println("[WARN] autopilot: UpgradeVersionTag %q defined but not set for all "+
					"servers, skipping migration check", conf.UpgradeVersionTag)
			}
		}
	}

	// Find any non-voters eligible for promotion
	var promotions []raft.Server
	for _, server := range servers {
		// Don't promote nodes marked non-voter
		if server.designatedNonVoter {
			continue
		}

		// If this server has been stable and passing for long enough, promote it to a voter
		if !autopilot.IsPotentialVoter(server.Suffrage) {
			// If there's already a voter in this node's zone, don't promote it
			if _, hasVoter := zoneVoters[server.redundancyZone]; hasVoter && server.redundancyZone != "" {
				continue
			}

			// If this server's version doesn't match desiredVersion, don't promote it
			if !conf.DisableUpgradeMigration && !server.version.Equal(migrationInfo.desiredVersion) {
				continue
			}

			// If stable and not a designated non-voter, add it to the eligible list
			if server.health.IsStable(now, conf) {
				promotions = append(promotions, server.Server)
				// Mark the zone as having a voter so we don't promote more than one
				// server in this zone.
				if server.redundancyZone != "" {
					zoneVoters[server.redundancyZone] = struct{}{}
				}
			}
		}
	}

	// If we have servers to promote, return early and skip demoting servers this run
	if len(promotions) > 0 {
		return promotions, nil
	}

	// Check for demotions if an upgrade is happening
	if migrationInfo.multipleVersions {
		// Limit demotions to one at a time, don't demote if there's a promotion (to avoid
		// losing leadership too soon), and don't demote anything until enough servers of the
		// desired version have been promoted.
		if len(migrationInfo.demotions) > 0 && migrationInfo.desiredVersionVoters >= migrationInfo.oldVoters {
			future := a.delegate.Raft().DemoteVoter(migrationInfo.demotions[0].ID, 0, 0)
			if err := future.Error(); err != nil {
				return nil, fmt.Errorf("failed to demote raft peer: %v", err)
			}
		}
	}

	return nil, nil
}

// migrationInfo contains info about the versions of the current server cluster
type migrationInfo struct {
	// desiredVersion is the Consul version autopilot is currently trying to migrate to, if applicable
	desiredVersion *version.Version

	// oldVoters is the count of voting servers not running desiredVersion
	oldVoters int

	// desiredVersionVoters is the count of voting servers running desiredVersion
	desiredVersionVoters int

	// multipleVersions is true if multiple versions of Consul are present in the cluster,
	// which makes us eligible for an upgrade migration scenario
	multipleVersions bool

	// demotions is the list of old voting servers that can be demoted after the migration
	demotions []raft.Server
}

// computeMigrationInfo takes the current servers autopilot is managing and returns
// version voter counts, the desired version, and any voter demotions
func computeMigrationInfo(servers map[raft.ServerID]*autopilotServer) migrationInfo {
	// Compute the minimum and maximum present versions in the cluster
	maxVersion, _ := version.NewVersion("0.0.0")
	versionCounts := make(map[string]int)
	for _, server := range servers {
		if server.version.GreaterThan(maxVersion) {
			maxVersion = server.version
		}
		versionCounts[server.version.String()]++
	}

	// Compute the desired version based on the version with the most servers,
	// preferring the higher version in a tie
	desiredVersion := maxVersion
	desiredVersionCount := versionCounts[maxVersion.String()]
	for ver, count := range versionCounts {
		if count > desiredVersionCount {
			desiredVersion, _ = version.NewVersion(ver)
			desiredVersionCount = count
		}
	}

	// Get any demotions and version voter counts
	var demotions []raft.Server
	oldVoters := 0
	desiredVersionVoters := 0
	for _, server := range servers {
		if server.version != nil && autopilot.IsPotentialVoter(server.Suffrage) {
			if !server.version.Equal(desiredVersion) {
				// Add old voters to demotions for when we finish promoting new servers
				oldVoters++
				demotions = append(demotions, server.Server)
			} else {
				desiredVersionVoters++
			}
		}
	}

	return migrationInfo{
		desiredVersion:       desiredVersion,
		oldVoters:            oldVoters,
		desiredVersionVoters: desiredVersionVoters,
		multipleVersions:     desiredVersionCount != len(servers),
		demotions:            demotions,
	}
}
