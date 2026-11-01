use std::collections::{HashMap, HashSet};

use crate::model::{TeamMembership, TeamScore, Totals};

pub(super) struct TeamEntry {
    pub(super) row: TeamScore,
    members: HashSet<i64>,
    pub(super) repositories: HashSet<String>,
}

pub(super) fn build_teams(memberships: &[TeamMembership]) -> (Vec<TeamEntry>, HashMap<i64, Vec<usize>>) {
    let mut teams: Vec<TeamEntry> = Vec::new();
    let mut team_index: HashMap<&str, usize> = HashMap::new();
    let mut member_teams: HashMap<i64, Vec<usize>> = HashMap::new();
    for membership in memberships {
        let index = *team_index.entry(membership.team_slug.as_str()).or_insert_with(|| {
            let index = teams.len();
            teams.push(TeamEntry {
                row: TeamScore {
                    team_slug: membership.team_slug.clone(),
                    team_name: membership.team_name.clone(),
                    member_count: 0,
                    totals: Totals::default(),
                    rank: 0,
                },
                members: HashSet::new(),
                repositories: HashSet::new(),
            });
            index
        });
        let team = &mut teams[index];
        if team.members.insert(membership.user_id) {
            member_teams.entry(membership.user_id).or_default().push(index);
        }
        if let Some(names) = &membership.repository_names {
            team.repositories.extend(names.iter().cloned());
        }
    }
    for team in &mut teams {
        team.row.member_count = u32::try_from(team.members.len()).unwrap_or(u32::MAX);
    }
    (teams, member_teams)
}
