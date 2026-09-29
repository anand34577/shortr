package io.github.anand34577.shortr.ui.links

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.CheckCircle
import androidx.compose.material.icons.rounded.Close
import androidx.compose.material.icons.rounded.ContentPaste
import androidx.compose.material.icons.rounded.ErrorOutline
import androidx.compose.material.icons.rounded.ExpandLess
import androidx.compose.material.icons.rounded.ExpandMore
import androidx.compose.material.icons.rounded.Visibility
import androidx.compose.material.icons.rounded.VisibilityOff
import androidx.compose.material3.AssistChip
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.InputChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import io.github.anand34577.shortr.container
import io.github.anand34577.shortr.data.Link
import io.github.anand34577.shortr.data.SessionState
import io.github.anand34577.shortr.ui.EditorRequest
import io.github.anand34577.shortr.ui.common.PrimaryWideButton
import io.github.anand34577.shortr.ui.common.appViewModel
import io.github.anand34577.shortr.ui.common.clipboardUrl
import io.github.anand34577.shortr.ui.common.shortDateTime
import io.github.anand34577.shortr.ui.theme.LocalExtraColors
import io.github.anand34577.shortr.ui.theme.MonoStyle
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.ZoneOffset
import java.time.temporal.ChronoUnit

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun LinkEditorSheet(request: EditorRequest, onDismiss: () -> Unit, onSaved: (Link, Boolean) -> Unit) {
    val ctx = LocalContext.current
    val c = ctx.container
    val key = request.existing?.id ?: ("new-" + request.initialUrl.orEmpty() + request.hashCode())
    val vm = appViewModel(key = "editor-$key") { LinkEditorViewModel(c, request.existing, request.initialUrl) }
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val host = (c.state.value as? SessionState.SignedIn)?.session?.server?.substringAfter("://") ?: ""
    var pickDate by remember { mutableStateOf(false) }
    var showPassword by remember { mutableStateOf(false) }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(
            Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding().navigationBarsPadding().padding(horizontal = 20.dp).padding(bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(if (vm.isEdit) "Edit link" else "New link", style = MaterialTheme.typography.titleLarge, modifier = Modifier.weight(1f))
                IconButton(onClick = onDismiss) { Icon(Icons.Rounded.Close, "Close") }
            }

            OutlinedTextField(
                value = vm.targetUrl,
                onValueChange = { vm.targetUrl = it; vm.error = null },
                label = { Text("Destination URL") },
                placeholder = { Text("https://example.com/a/very/long/page") },
                isError = vm.fieldErrors["targetUrl"] != null,
                supportingText = vm.fieldErrors["targetUrl"]?.let { { Text(it) } },
                trailingIcon = {
                    IconButton(onClick = { ctx.clipboardUrl()?.let { vm.targetUrl = it } }) { Icon(Icons.Rounded.ContentPaste, "Paste link from clipboard") }
                },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Next, autoCorrectEnabled = false),
                maxLines = 3,
                modifier = Modifier.fillMaxWidth(),
            )

            OutlinedTextField(
                value = vm.code,
                onValueChange = vm::onCode,
                label = { Text(if (vm.isEdit) "Short code" else "Custom alias (optional)") },
                prefix = { Text("$host/", color = MaterialTheme.colorScheme.onSurfaceVariant) },
                textStyle = MonoStyle,
                singleLine = true,
                isError = vm.fieldErrors["code"] != null || vm.aliasState == AliasState.Taken || vm.aliasState == AliasState.Invalid,
                supportingText = {
                    val msg = vm.fieldErrors["code"] ?: when (vm.aliasState) {
                        AliasState.Taken -> "Already taken"
                        AliasState.Invalid -> "Letters, numbers, - and _ only; some words are reserved"
                        AliasState.Available -> "Available"
                        else -> if (vm.isEdit) "Changing it breaks the old short link" else "Leave empty for a random code"
                    }
                    Text(msg)
                },
                trailingIcon = {
                    when (vm.aliasState) {
                        AliasState.Checking -> CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        AliasState.Available -> Icon(Icons.Rounded.CheckCircle, null, tint = LocalExtraColors.current.success)
                        AliasState.Taken, AliasState.Invalid -> Icon(Icons.Rounded.ErrorOutline, null, tint = MaterialTheme.colorScheme.error)
                        AliasState.Idle -> Unit
                    }
                },
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next, autoCorrectEnabled = false),
                modifier = Modifier.fillMaxWidth(),
            )

            OutlinedTextField(
                value = vm.title,
                onValueChange = { vm.title = it.take(200) },
                label = { Text("Title (optional)") },
                placeholder = { Text("Fetched from the page if left empty") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = vm.tagInput,
                    onValueChange = { v -> if (v.endsWith(",")) vm.addTag().also { vm.tagInput = "" } else vm.tagInput = v },
                    label = { Text("Tags") },
                    placeholder = { Text("Type and press enter") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                    keyboardActions = KeyboardActions(onDone = { vm.addTag() }),
                    modifier = Modifier.fillMaxWidth(),
                )
                if (vm.tags.isNotEmpty()) {
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        vm.tags.forEach { t ->
                            InputChip(selected = false, onClick = { vm.removeTag(t) }, label = { Text(t) },
                                trailingIcon = { Icon(Icons.Rounded.Close, "Remove tag $t", Modifier.size(16.dp)) })
                        }
                    }
                }
            }

            TextButton(onClick = { vm.showAdvanced = !vm.showAdvanced }) {
                Text(if (vm.showAdvanced) "Fewer options" else "Expiry, password and more")
                Icon(if (vm.showAdvanced) Icons.Rounded.ExpandLess else Icons.Rounded.ExpandMore, null)
            }

            AnimatedVisibility(vm.showAdvanced) {
                Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                    // Expiry
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text("Expires", style = MaterialTheme.typography.labelLarge)
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                            FilterChip(selected = vm.expiresAt == null, onClick = { vm.expiresAt = null }, label = { Text("Never") })
                            listOf(1L to "1 day", 7L to "7 days", 30L to "30 days").forEach { (d, label) ->
                                FilterChip(selected = false, onClick = { vm.expiresAt = Instant.now().plus(d, ChronoUnit.DAYS).truncatedTo(ChronoUnit.MINUTES) }, label = { Text(label) })
                            }
                            AssistChip(onClick = { pickDate = true }, label = { Text("Pick date") })
                        }
                        vm.expiresAt?.let { Text("Stops working ${shortDateTime(it.toString())}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                        vm.fieldErrors["expiresAt"]?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error) }
                    }

                    OutlinedTextField(
                        value = vm.maxClicks,
                        onValueChange = { vm.maxClicks = it.filter(Char::isDigit).take(9) },
                        label = { Text("Click limit (optional)") },
                        placeholder = { Text("e.g. 100") },
                        isError = vm.fieldErrors["maxClicks"] != null,
                        supportingText = { Text(vm.fieldErrors["maxClicks"] ?: "The link stops working after this many visits") },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )

                    // Password
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text("Password", style = MaterialTheme.typography.labelLarge)
                        if (vm.hadPassword) {
                            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                                PasswordMode.entries.forEachIndexed { i, m ->
                                    SegmentedButton(selected = vm.passwordMode == m, onClick = { vm.passwordMode = m }, shape = SegmentedButtonDefaults.itemShape(i, 3)) {
                                        Text(when (m) { PasswordMode.Keep -> "Keep"; PasswordMode.Set -> "Change"; PasswordMode.Remove -> "Remove" })
                                    }
                                }
                            }
                        }
                        if (vm.passwordMode == PasswordMode.Set) {
                            OutlinedTextField(
                                value = vm.password,
                                onValueChange = { vm.password = it.take(128) },
                                placeholder = { Text("Visitors must enter this first") },
                                singleLine = true,
                                visualTransformation = if (showPassword) VisualTransformation.None else PasswordVisualTransformation(),
                                trailingIcon = {
                                    IconButton(onClick = { showPassword = !showPassword }) {
                                        Icon(if (showPassword) Icons.Rounded.VisibilityOff else Icons.Rounded.Visibility, if (showPassword) "Hide password" else "Show password")
                                    }
                                },
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }

                    // Redirect type
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text("Redirect type", style = MaterialTheme.typography.labelLarge)
                        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                            SegmentedButton(selected = !vm.permanent, onClick = { vm.permanent = false }, shape = SegmentedButtonDefaults.itemShape(0, 2)) { Text("Temporary") }
                            SegmentedButton(selected = vm.permanent, onClick = { vm.permanent = true }, shape = SegmentedButtonDefaults.itemShape(1, 2)) { Text("Permanent") }
                        }
                        Text(
                            if (vm.permanent) "Browsers may cache it, so later changes and some clicks won't be seen." else "Every visit is counted and you can change the destination any time.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }

                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("Forward query string", style = MaterialTheme.typography.bodyLarge)
                            Text("?utm=… added to the short link is passed on", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Switch(checked = vm.passQuery, onCheckedChange = { vm.passQuery = it })
                    }
                }
            }

            vm.error?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium) }
            PrimaryWideButton(if (vm.isEdit) "Save changes" else "Create short link", onClick = { vm.save(onSaved) }, loading = vm.saving)
        }
    }

    if (pickDate) {
        val initial = (vm.expiresAt ?: Instant.now().plus(7, ChronoUnit.DAYS)).toEpochMilli()
        val state = rememberDatePickerState(
            initialSelectedDateMillis = initial,
            selectableDates = object : androidx.compose.material3.SelectableDates {
                override fun isSelectableDate(utcTimeMillis: Long) = utcTimeMillis >= LocalDate.now().atStartOfDay().toInstant(ZoneOffset.UTC).toEpochMilli()
            },
        )
        DatePickerDialog(
            onDismissRequest = { pickDate = false },
            confirmButton = {
                TextButton(onClick = {
                    state.selectedDateMillis?.let { ms ->
                        // end of the chosen day in the phone's time zone
                        val day = Instant.ofEpochMilli(ms).atZone(ZoneOffset.UTC).toLocalDate()
                        vm.expiresAt = day.atTime(LocalTime.of(23, 59)).atZone(ZoneId.systemDefault()).toInstant()
                    }
                    pickDate = false
                }) { Text("Set") }
            },
            dismissButton = { TextButton(onClick = { pickDate = false }) { Text("Cancel") } },
        ) { DatePicker(state) }
    }
}
