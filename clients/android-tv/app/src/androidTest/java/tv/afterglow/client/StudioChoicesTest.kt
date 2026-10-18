package tv.afterglow.client
import androidx.compose.runtime.*
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.input.key.Key
import org.junit.Rule
import org.junit.Test
@OptIn(ExperimentalTestApi::class)
class StudioChoicesTest {
 @get:Rule val compose=createComposeRule()
 @Test fun choicesNavigateAcrossColumnsWithoutTextInput(){
  compose.setContent{Column{var selected by remember{mutableStateOf("")};StudioChoices((1..12).map{it.toString() to "Choice $it"},selected){selected=it}}}
  compose.onAllNodes(hasSetTextAction()).assertCountEquals(0)
  compose.onNodeWithText("Choice 1").performSemanticsAction(SemanticsActions.RequestFocus)
  compose.onNodeWithText("Choice 1").performKeyInput{pressKey(Key.DirectionRight)}
  compose.onNodeWithText("Choice 2").assertIsFocused()
  compose.onNodeWithText("Choice 2").performKeyInput{pressKey(Key.DirectionDown)}
  compose.onNodeWithText("Choice 5").assertIsFocused()
  compose.onNodeWithText("Choice 5").performSemanticsAction(SemanticsActions.OnClick)
  compose.onNodeWithText("✓ Choice 5").assertExists()
 }
}
