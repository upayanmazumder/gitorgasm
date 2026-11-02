import { Button } from '@/components/ui';

import type { Kind } from '../data';
import { blockedNote, blockedReason, personNote, type Actor, type Person } from '../draft';

interface Props {
  org: string;
  kind: Kind;
  actor: Actor;
  people: readonly Person[];
  max: number;
  onChange: (people: Person[]) => void;
  focus: (target: string) => void;
  register: (target: string) => (element: HTMLElement | null) => void;
}

export function SelectedPeople({
  org,
  kind,
  actor,
  people,
  max,
  onChange,
  focus,
  register,
}: Props) {
  function remove(index: number) {
    // Focus the neighbor so keyboard users keep their place.
    const neighbor = people[index + 1] ?? people[index - 1];
    onChange(people.filter((_, position) => position !== index));
    focus(neighbor ? `remove:${neighbor.key}` : 'selected');
  }

  return (
    <div className="space-y-4">
      <h3 tabIndex={-1} ref={register('selected')}>
        Selected people
      </h3>
      <p role="status" className="tabular-nums">
        {people.length} of {max} selected
      </p>
      {people.length === 0 ? (
        <p className="text-muted-foreground">No one selected yet.</p>
      ) : (
        <ul aria-label="Selected people" className="divide-y divide-border border-y border-border">
          {people.map((person, index) => {
            const blocked = blockedReason(person, kind, actor);
            const note = blocked ? blockedNote(blocked, org) : personNote(person, kind, org);
            return (
              <li
                key={person.key}
                className="flex flex-col gap-2 py-3 md:flex-row md:items-center md:justify-between"
              >
                <div>
                  <span className="font-semibold wrap-anywhere">{person.login}</span>
                  {note && (
                    <span
                      className={blocked ? 'block text-sm' : 'block text-sm text-muted-foreground'}
                    >
                      {note}
                    </span>
                  )}
                </div>
                <Button
                  variant="outline"
                  aria-label={`Remove ${person.login}`}
                  ref={register(`remove:${person.key}`)}
                  onClick={() => remove(index)}
                >
                  Remove
                </Button>
              </li>
            );
          })}
        </ul>
      )}
      {people.length > 0 && (
        <Button
          variant="outline"
          onClick={() => {
            onChange([]);
            focus('selected');
          }}
        >
          Clear list
        </Button>
      )}
    </div>
  );
}
