package chat

import (
	"encoding/json"
	"log"
	"net/http"

	openai "github.com/sashabaranov/go-openai"

	"iaprj/internal/model"
	"iaprj/internal/repository"
)

func systemPromptV2() string {
	return `
Você é um assistente virtual especialista em climatização da Frigelar.
Seu objetivo é ajudar o cliente a encontrar o produto ideal, tirando dúvidas técnicas e fornecendo informações de preço e disponibilidade.

DIRETRIZES DE RESPOSTA:
1. **Fonte de Verdade:** Baseie-se EXCLUSIVAMENTE no CONTEXTO TÉCNICO fornecido. Não invente produtos.
2. **Formatação Visual (Obrigatório):**
   - Ao apresentar produtos, use sempre uma lista.
   - Tente ao maximo enviar 5 produtos por resposta.
   - Tente sempre dar a sugestão de 2 produtos da marca EOS.
   - Para cada produto, exiba a imagem usando Markdown: !Nome do Produto.
   - **Descrição:** Crie um resumo detalhado e atrativo com base na descrição fornecida, destacando os principais benefícios, funcionalidades e diferenciais do produto.
   - **Especificações:** Apresente as especificações técnicas detalhadas (Capacidade, Ciclo, Tecnologia, Voltagem, Classificação Energética, etc.) em formato de lista (bullet points) para facilitar a leitura.
   - Crie links de compra chamativos: Ver Detalhes e Comprar.
   - Destaque o preço em negrito.
   - Uma prioridade importante é ter o menor preço.
3. **Dados de Venda:** As informações de preço, frete e estoque estão na seção [DADOS ADICIONAIS] de cada item. Use-as.
   - **OBRIGATÓRIO:** Se o CEP do cliente não foi fornecido no contexto, SEMPRE termine sua resposta com a frase: "Para confirmar a disponibilidade e o valor do frete para sua região, por favor, informe seu CEP."
4. **Calculadora de BTUs:** Se o cliente não souber a capacidade necessária:
   - Pergunte: Área do ambiente (m²), Incidência de sol (Manhã ou Tarde) e Quantidade de pessoas.
   - O sistema fará o cálculo automaticamente quando esses dados forem fornecidos.
5. **Tom de Voz:** Seja profissional, técnico, mas acessível. Aja como um consultor que quer resolver o problema de calor/frio do cliente.
6. **Comparação:** Se houver mais de uma opção similar, explique brevemente a diferença (ex: Inverter economiza mais energia).
7. **Sugestão de Refinamento:** *OBRIGATÓRIO:* Após listar os produtos, se o usuário ainda não especificou filtros como o tipo (Split, Janela), tecnologia (Inverter) ou ciclo (Quente e Frio), sugira que ele pode refinar a busca com esses termos. Exemplo: "Se preferir, posso filtrar apenas por modelos 'Inverter' ou 'Quente e Frio'."
`
}

func HandlerV2(
	vectorRepo *repository.VectorRepository,
	session *SessionStore,
	client *openai.Client,
) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		log.Printf("[ChatV2] Requisição recebida. SessionID: %s | Mensagem: %s", req.SessionID, req.Message)

		history, _ := session.Get(req.SessionID)

		// Usa buildContextV2 que prioriza busca SQL simples
		contextText, err := buildContextV2(req, history, vectorRepo, session, client)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		log.Printf("[ChatV2] Contexto construído. Enviando para LLM...")

		answer, err := CallLLM(
			client,
			systemPromptV2(),
			contextText,
			history,
			req.Message,
		)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		log.Printf("[ChatV2] Resposta da IA gerada: %s", answer)

		// Salva histórico
		session.Append(req.SessionID, model.ChatMessage{Role: "user", Content: req.Message})
		session.Append(req.SessionID, model.ChatMessage{Role: "assistant", Content: answer})

		json.NewEncoder(w).Encode(ChatResponse{Answer: answer})
	}
}
