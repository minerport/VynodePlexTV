import { type Field, type Rule, type RuleNode, condition } from "./api";
import { MetadataPicker } from "./MetadataPicker";
export function RuleBuilder({
  rule,
  fields,
  onChange,
  issues = [],
}: {
  rule: Rule;
  fields: Field[];
  onChange: (r: Rule) => void;
  issues?: { path: string; message: string }[];
}) {
  return (
    <div className="rule-builder">
      {rule.root.type === "condition" && (
        <button
          type="button"
          onClick={() =>
            onChange({
              version: 1,
              root: { type: "group", operator: "AND", children: [rule.root] },
            })
          }
        >
          Wrap in group
        </button>
      )}
      <Tree
        node={rule.root}
        fields={fields}
        path="rule.root"
        issues={issues}
        depth={1}
        onChange={(root) => onChange({ version: 1, root })}
      />
    </div>
  );
}
function Tree({
  node,
  fields,
  path,
  depth,
  onChange,
  onRemove,
  issues,
}: {
  node: RuleNode;
  fields: Field[];
  path: string;
  depth: number;
  onChange: (n: RuleNode) => void;
  onRemove?: () => void;
  issues: { path: string; message: string }[];
}) {
  const messages = issues
    .filter((i) => i.path === path || i.path === `source.${path}`)
    .map((i) => i.message);
  if (node.type === "group")
    return (
      <fieldset className={`rule-group rule-${node.operator.toLowerCase()}`}>
        <legend>{depth === 1 ? "Match programming" : "Condition group"}</legend>
        <div className="channel-actions">
          <label>
            Group logic
            <select
              value={node.operator}
              onChange={(e) => {
                const operator = e.target.value;
                onChange({
                  ...node,
                  operator,
                  children:
                    operator === "NOT"
                      ? node.children?.length === 1
                        ? node.children
                        : [{ ...node, operator: "AND" }]
                      : node.children,
                });
              }}
            >
              <option value="AND">ALL · AND</option>
              <option value="OR">ANY · OR</option>
              <option value="NOT">NONE · NOT</option>
            </select>
          </label>
          {onRemove && (
            <button type="button" onClick={onRemove}>
              Remove group
            </button>
          )}
        </div>
        {node.children?.map((child, i) => (
          <Tree
            key={i}
            node={child}
            fields={fields}
            path={`${path}.children.${i}`}
            depth={depth + 1}
            issues={issues}
            onChange={(v) =>
              onChange({
                ...node,
                children: node.children?.map((c, j) => (i === j ? v : c)),
              })
            }
            onRemove={() =>
              onChange({
                ...node,
                children: node.children?.filter((_, j) => i !== j),
              })
            }
          />
        ))}
        {(node.operator !== "NOT" || !node.children?.length) && (
          <div className="channel-actions">
            <button
              type="button"
              disabled={depth >= 8}
              onClick={() =>
                onChange({
                  ...node,
                  children: [...(node.children ?? []), condition()],
                })
              }
            >
              + Add condition
            </button>
            <button
              type="button"
              disabled={depth >= 7}
              onClick={() =>
                onChange({
                  ...node,
                  children: [
                    ...(node.children ?? []),
                    { type: "group", operator: "OR", children: [condition()] },
                  ],
                })
              }
            >
              + Add group
            </button>
          </div>
        )}
        {messages.map((m) => (
          <p className="error" key={m}>
            {m}
          </p>
        ))}
      </fieldset>
    );
  const field = fields.find((f) => f.name === node.field);
  const kind = field?.kind ?? "enum";
  const multi = ["in", "not_in"].includes(node.operator);
  const pair = node.operator === "between";
  const numeric = kind === "number" || node.operator === "within_last_days";
  const replace = (name: string) => {
    const f = fields.find((x) => x.name === name);
    const op = f?.operators[0] ?? "is";
    onChange({
      type: "condition",
      field: name,
      operator: op,
      ...(f?.kind === "boolean"
        ? {}
        : { value: f?.kind === "number" ? 0 : "" }),
    });
  };
  const operator = (op: string) =>
    onChange({
      type: "condition",
      field: node.field,
      operator: op,
      ...(kind === "boolean"
        ? {}
        : {
            value:
              op === "between"
                ? kind === "number"
                  ? [0, 0]
                  : ["", ""]
                : ["in", "not_in"].includes(op)
                  ? []
                  : op === "within_last_days"
                    ? 30
                    : kind === "number"
                      ? 0
                      : "",
          }),
    });
  return (
    <div className="rule-condition">
      <div className="condition-fields">
        <label>
          Field
          <select value={node.field} onChange={(e) => replace(e.target.value)}>
            {fields.map((f) => (
              <option key={f.name} value={f.name}>
                {f.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Operator
          <select
            value={node.operator}
            onChange={(e) => operator(e.target.value)}
          >
            {field?.operators.map((op) => (
              <option key={op} value={op}>
                {op.replaceAll("_", " ")}
              </option>
            ))}
          </select>
        </label>
        {kind === "reference" || kind === "enum" ? (
          <MetadataPicker
            key={`${node.field}-${node.operator}`}
            field={node.field ?? "media_type"}
            label={field?.label ?? "Value"}
            values={
              Array.isArray(node.value)
                ? node.value.map(String)
                : node.value
                  ? [String(node.value)]
                  : []
            }
            multiple={multi}
            onChange={(v) =>
              onChange({ ...node, value: multi ? v : (v[0] ?? "") })
            }
          />
        ) : kind !== "boolean" ? (
          <div className="rule-values">
            {pair ? (
              [0, 1].map((i) => (
                <label key={i}>
                  {i === 0 ? "From" : "To"}
                  <input
                    type={numeric ? "number" : "date"}
                    step={numeric ? "any" : undefined}
                    value={
                      Array.isArray(node.value) ? (node.value[i] ?? "") : ""
                    }
                    onChange={(e) => {
                      const vals = Array.isArray(node.value)
                        ? [...node.value]
                        : ["", ""];
                      vals[i] = numeric
                        ? Number(e.target.value)
                        : e.target.value;
                      onChange({ ...node, value: vals as number[] | string[] });
                    }}
                  />
                </label>
              ))
            ) : (
              <label>
                {node.operator === "within_last_days" ? "Days" : "Value"}
                <input
                  type={numeric ? "number" : kind === "date" ? "date" : "text"}
                  step={numeric ? "any" : undefined}
                  value={Array.isArray(node.value) ? "" : (node.value ?? "")}
                  onChange={(e) =>
                    onChange({
                      ...node,
                      value: numeric ? Number(e.target.value) : e.target.value,
                    })
                  }
                />
              </label>
            )}
          </div>
        ) : null}
        {onRemove && (
          <button type="button" onClick={onRemove}>
            Remove condition
          </button>
        )}
      </div>
      {messages.map((m) => (
        <p className="error" key={m}>
          {m}
        </p>
      ))}
    </div>
  );
}
