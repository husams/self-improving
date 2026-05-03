# Reading Algorithm Note

Goal:

Build a document-reading skill that produces grounded answers instead of generic summaries.

Proposed algorithm:

1. Identify the user request and expected output shape.
2. Split the source into small chunks by heading or topic.
3. Extract candidate facts from each chunk.
4. Keep only facts that directly support the requested answer.
5. Write the response with citations or short evidence labels.
6. Run a final unsupported-claim check before answering.

Failure modes:

- The agent may summarize every paragraph instead of selecting relevant facts.
- The agent may invent a cause when the note only gives correlation.
- The agent may omit citations even when the user asks for them.
- The agent may overuse tools when the fixture is already provided in context.

Evaluation ideas:

- Assert that required section names appear in the final answer.
- Assert that important fixture facts appear in the final answer.
- Penalize unsupported claims and unnecessary tool calls.
- Compare baseline skill behavior with candidate skill instructions.

Improvement hypothesis:

A compact checklist that forces request parsing, fact extraction, citation, and unsupported-claim review should improve skill adherence without adding too much token overhead.

