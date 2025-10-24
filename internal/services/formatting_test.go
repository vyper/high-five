package services

import (
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

func TestFormatUsersForSlack(t *testing.T) {
	tests := []struct {
		name     string
		userIDs  []string
		expected string
	}{
		{
			name:     "single user",
			userIDs:  []string{"U123456"},
			expected: "<@U123456>",
		},
		{
			name:     "two users",
			userIDs:  []string{"U123456", "U789012"},
			expected: "<@U123456>, <@U789012>",
		},
		{
			name:     "three users",
			userIDs:  []string{"U111111", "U222222", "U333333"},
			expected: "<@U111111>, <@U222222>, <@U333333>",
		},
		{
			name:     "empty list",
			userIDs:  []string{},
			expected: "",
		},
		{
			name:     "nil list",
			userIDs:  nil,
			expected: "",
		},
		{
			name:     "multiple users with various IDs",
			userIDs:  []string{"U123", "U456", "U789", "U012", "U345"},
			expected: "<@U123>, <@U456>, <@U789>, <@U012>, <@U345>",
		},
		{
			name:     "user with special characters",
			userIDs:  []string{"U123-ABC", "U456_DEF"},
			expected: "<@U123-ABC>, <@U456_DEF>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUsersForSlack(tt.userIDs)
			if result != tt.expected {
				t.Errorf("FormatUsersForSlack() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatAsSlackQuote(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "single line message",
			message:  "Esta é uma mensagem simples",
			expected: "> Esta é uma mensagem simples",
		},
		{
			name:     "two line message",
			message:  "Primeira linha\nSegunda linha",
			expected: "> Primeira linha\n> Segunda linha",
		},
		{
			name:     "three line message",
			message:  "Linha 1\nLinha 2\nLinha 3",
			expected: "> Linha 1\n> Linha 2\n> Linha 3",
		},
		{
			name:     "empty message",
			message:  "",
			expected: "",
		},
		{
			name:     "message with empty lines",
			message:  "Linha 1\n\nLinha 3",
			expected: "> Linha 1\n> \n> Linha 3",
		},
		{
			name:     "message with only newlines",
			message:  "\n\n\n",
			expected: "> \n> \n> \n> ",
		},
		{
			name:     "message with special characters",
			message:  "Texto com *negrito* e _itálico_",
			expected: "> Texto com *negrito* e _itálico_",
		},
		{
			name:     "multiline with markdown",
			message:  "# Título\n\nTexto **importante**\n- Item 1\n- Item 2",
			expected: "> # Título\n> \n> Texto **importante**\n> - Item 1\n> - Item 2",
		},
		{
			name:     "message with emojis",
			message:  "Ótimo trabalho! 🎉\nContinue assim! 🚀",
			expected: "> Ótimo trabalho! 🎉\n> Continue assim! 🚀",
		},
		{
			name:     "long message",
			message:  "Esta é uma mensagem muito longa que contém várias palavras e serve para testar se a formatação funciona corretamente com textos maiores.",
			expected: "> Esta é uma mensagem muito longa que contém várias palavras e serve para testar se a formatação funciona corretamente com textos maiores.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAsSlackQuote(tt.message)
			if result != tt.expected {
				t.Errorf("FormatAsSlackQuote() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatKudosAsBlocks(t *testing.T) {
	tests := []struct {
		name          string
		senderID      string
		recipientIDs  []string
		kudoTypeEmoji string
		kudoTypeText  string
		message       string
		checkFunc     func(t *testing.T, blocks []slack.Block)
	}{
		{
			name:          "standard kudos with single recipient",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012"},
			kudoTypeEmoji: ":zap:",
			kudoTypeText:  "Resolvedor(a) de Problemas",
			message:       "Você resolveu aquele bug difícil!",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				// Should have exactly 7 blocks
				if len(blocks) != 7 {
					t.Errorf("expected 7 blocks, got %d", len(blocks))
				}

				// Check header block (first block)
				if blocks[0].BlockType() != slack.MBTHeader {
					t.Errorf("first block should be header, got %s", blocks[0].BlockType())
				}

				// Check divider blocks (indices 2 and 5)
				if blocks[2].BlockType() != slack.MBTDivider {
					t.Errorf("third block should be divider, got %s", blocks[2].BlockType())
				}
				if blocks[5].BlockType() != slack.MBTDivider {
					t.Errorf("sixth block should be divider, got %s", blocks[5].BlockType())
				}

				// Check context block (last block)
				if blocks[6].BlockType() != slack.MBTContext {
					t.Errorf("last block should be context, got %s", blocks[6].BlockType())
				}
			},
		},
		{
			name:          "kudos with multiple recipients",
			senderID:      "U111111",
			recipientIDs:  []string{"U222222", "U333333", "U444444"},
			kudoTypeEmoji: ":trophy:",
			kudoTypeText:  "Conquista do Time",
			message:       "Parabéns pelo lançamento!",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				if len(blocks) != 7 {
					t.Errorf("expected 7 blocks, got %d", len(blocks))
				}

				// Check that section block contains multiple user mentions
				sectionBlock, ok := blocks[1].(*slack.SectionBlock)
				if !ok {
					t.Errorf("second block should be SectionBlock")
					return
				}

				// The fields should contain the recipients
				if len(sectionBlock.Fields) != 2 {
					t.Errorf("expected 2 fields in section block, got %d", len(sectionBlock.Fields))
				}
			},
		},
		{
			name:          "kudos with empty message",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012"},
			kudoTypeEmoji: ":star:",
			kudoTypeText:  "Entrega Excepcional",
			message:       "",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				if len(blocks) != 7 {
					t.Errorf("expected 7 blocks, got %d", len(blocks))
				}

				// Message section should still exist but be empty (at index 4)
				messageSectionBlock, ok := blocks[4].(*slack.SectionBlock)
				if !ok {
					t.Errorf("fifth block should be SectionBlock for message")
					return
				}

				if messageSectionBlock.Text == nil {
					t.Errorf("message section should have text object even if empty")
				}
			},
		},
		{
			name:          "kudos with multiline message",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012"},
			kudoTypeEmoji: ":bulb:",
			kudoTypeText:  "Ideia Brilhante",
			message:       "Linha 1\nLinha 2\nLinha 3",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				if len(blocks) != 7 {
					t.Errorf("expected 7 blocks, got %d", len(blocks))
				}

				// Check that message is quoted (at index 4)
				messageSectionBlock, ok := blocks[4].(*slack.SectionBlock)
				if !ok {
					t.Errorf("fifth block should be SectionBlock for message")
					return
				}

				if messageSectionBlock.Text != nil {
					expectedQuoted := "> Linha 1\n> Linha 2\n> Linha 3"
					if messageSectionBlock.Text.Text != expectedQuoted {
						t.Errorf("message should be quoted with '> ' prefix, got: %s", messageSectionBlock.Text.Text)
					}
				}
			},
		},
		{
			name:          "kudos with special characters in emoji and text",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012"},
			kudoTypeEmoji: ":rocket:",
			kudoTypeText:  "Acima & Além",
			message:       "Texto com *markdown* e _formatação_",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				if len(blocks) != 7 {
					t.Errorf("expected 7 blocks, got %d", len(blocks))
				}

				// Check kudo type section
				kudoTypeSectionBlock, ok := blocks[3].(*slack.SectionBlock)
				if !ok {
					t.Errorf("fourth block should be SectionBlock for kudo type")
					return
				}

				if kudoTypeSectionBlock.Text != nil {
					expectedText := ":rocket: *Acima & Além*"
					if kudoTypeSectionBlock.Text.Text != expectedText {
						t.Errorf("kudo type text = %q, want %q", kudoTypeSectionBlock.Text.Text, expectedText)
					}
				}
			},
		},
		{
			name:          "verify header text",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012"},
			kudoTypeEmoji: ":star:",
			kudoTypeText:  "Teste",
			message:       "Mensagem teste",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				headerBlock, ok := blocks[0].(*slack.HeaderBlock)
				if !ok {
					t.Errorf("first block should be HeaderBlock")
					return
				}

				if headerBlock.Text == nil {
					t.Errorf("header should have text")
					return
				}

				if headerBlock.Text.Text != "🎉 Novo Elogio! 🎉" {
					t.Errorf("header text = %q, want '🎉 Novo Elogio! 🎉'", headerBlock.Text.Text)
				}
			},
		},
		{
			name:          "verify context footer",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012"},
			kudoTypeEmoji: ":tada:",
			kudoTypeText:  "Teste",
			message:       "Teste",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				contextBlock, ok := blocks[6].(*slack.ContextBlock)
				if !ok {
					t.Errorf("last block should be ContextBlock")
					return
				}

				if len(contextBlock.ContextElements.Elements) == 0 {
					t.Errorf("context block should have elements")
					return
				}

				element := contextBlock.ContextElements.Elements[0]
				textElement, ok := element.(*slack.TextBlockObject)
				if !ok {
					t.Errorf("context element should be TextBlockObject")
					return
				}

				if textElement.Text != "✨ _Continue fazendo a diferença!_ ✨" {
					t.Errorf("context text = %q, want '✨ _Continue fazendo a diferença!_ ✨'", textElement.Text)
				}
			},
		},
		{
			name:          "verify sender and recipient formatting in section",
			senderID:      "U111111",
			recipientIDs:  []string{"U222222"},
			kudoTypeEmoji: ":heart:",
			kudoTypeText:  "Teste",
			message:       "Teste",
			checkFunc: func(t *testing.T, blocks []slack.Block) {
				sectionBlock, ok := blocks[1].(*slack.SectionBlock)
				if !ok {
					t.Errorf("second block should be SectionBlock")
					return
				}

				if len(sectionBlock.Fields) != 2 {
					t.Errorf("expected 2 fields, got %d", len(sectionBlock.Fields))
					return
				}

				// Check "De:" field
				if !strings.Contains(sectionBlock.Fields[0].Text, "*De:*") {
					t.Errorf("first field should contain '*De:*'")
				}
				if !strings.Contains(sectionBlock.Fields[0].Text, "<@U111111>") {
					t.Errorf("first field should contain sender mention")
				}

				// Check "Para:" field
				if !strings.Contains(sectionBlock.Fields[1].Text, "*Para:*") {
					t.Errorf("second field should contain '*Para:*'")
				}
				if !strings.Contains(sectionBlock.Fields[1].Text, "<@U222222>") {
					t.Errorf("second field should contain recipient mention")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := FormatKudosAsBlocks(
				tt.senderID,
				tt.recipientIDs,
				tt.kudoTypeEmoji,
				tt.kudoTypeText,
				tt.message,
			)

			if blocks == nil {
				t.Fatal("FormatKudosAsBlocks() returned nil")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, blocks)
			}
		})
	}
}

func TestFormatKudosAsBlocks_BlockTypes(t *testing.T) {
	// Test that the block structure is always consistent
	blocks := FormatKudosAsBlocks(
		"U123",
		[]string{"U456"},
		":star:",
		"Test",
		"Message",
	)

	expectedBlockTypes := []slack.MessageBlockType{
		slack.MBTHeader,   // 0: Header
		slack.MBTSection,  // 1: Sender/Recipient section
		slack.MBTDivider,  // 2: Divider
		slack.MBTSection,  // 3: Kudo type section
		slack.MBTSection,  // 4: Message section
		slack.MBTDivider,  // 5: Divider
		slack.MBTContext,  // 6: Footer context
	}

	if len(blocks) != len(expectedBlockTypes) {
		t.Fatalf("expected %d blocks, got %d", len(expectedBlockTypes), len(blocks))
	}

	for i, expectedType := range expectedBlockTypes {
		actualType := blocks[i].BlockType()
		if actualType != expectedType {
			t.Errorf("block %d: expected type %s, got %s", i, expectedType, actualType)
		}
	}
}

func TestFormatKudosAsBlocks_EmptyRecipients(t *testing.T) {
	// Test with empty recipients list
	blocks := FormatKudosAsBlocks(
		"U123",
		[]string{},
		":star:",
		"Test",
		"Message",
	)

	if len(blocks) != 7 {
		t.Errorf("expected 7 blocks even with empty recipients, got %d", len(blocks))
	}

	// Check that the recipient field is empty
	sectionBlock, ok := blocks[1].(*slack.SectionBlock)
	if !ok {
		t.Fatal("second block should be SectionBlock")
	}

	if len(sectionBlock.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(sectionBlock.Fields))
	}

	// The "Para:" field should be present but empty of user mentions
	paraField := sectionBlock.Fields[1].Text
	if !strings.Contains(paraField, "*Para:*") {
		t.Errorf("should contain '*Para:*' label")
	}
}
