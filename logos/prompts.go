package logos

import (
	"fmt"
)

func GetArchitectPrompt(content string) string {
	return fmt.Sprintf(`You are a Software Architecture Analyst.
Read the code and extract:
1. File objective.
2. Main Functions.
3. State Variables/Structures.
DO NOT include code.

Code:
%s`, content)
}

func GetSystemPrompt(action, cacheSection, content, instruction string) (string, error) {
	var role, taskContext string

	switch action {
	case "feat":
		role = "You are a Creative and Focused Senior Developer."
		if content == "" {
			taskContext = "CREATE FILE: Write complete code from scratch based on the instruction below."
		} else {
			taskContext = "ADD FEATURE: Insert the requested code into the appropriate logical location of the existing structure."
		}
	case "fix":
		role = "You are a Quality Assurance (QA) Engineer expert in Debugging."
		taskContext = "BUG FIX: Analyze the code, find the reported failure in the instruction, and modify only the targeted problem area to resolve it."
	case "refactor":
		role = "You are a Software Architecture Purist (Clean Code)."
		taskContext = "REFACTOR: Optimize code structure, readability, and performance. DO NOT add new features or break current behavior."
	case "doc":
		role = "You are a Technical Documentation Engineer (Tech Writer)."
		taskContext = "DOCUMENTATION: Add useful comments, docstrings, or GoDoc formats on core functions and structures. Do not change the application logic."
	default:
		return "", fmt.Errorf("invalid action detected: '%s'", action)
	}

	prompt := fmt.Sprintf(`<role>%s</role>
<context>
ACTION: %s
%s
</context>

<instructions>
Return STRICTLY in the XML structure below. DO NOT add any text outside the XML tags.
<progress>
List SPECIFICALLY: which logic parts were changed and the technical reason. Generic answers are FORBIDDEN.
</progress>
<code>
[COMPLETE FINAL CODE HERE. 
CRITICAL RULE 1: DO NOT ESCAPE HTML CHARACTERS.
CRITICAL RULE 2: DO NOT USE MARKDOWN BLOCKS inside here. JUST RAW CODE.
CRITICAL RULE 3: GENERATE ONLY THE CONTENT OF THE REQUESTED FILE.]
</code>
</instructions>

<original_code>
%s
</original_code>

<instruction>
%s
</instruction>`, role, taskContext, cacheSection, content, instruction)

	return prompt, nil
}